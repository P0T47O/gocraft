//go:build windows

package main

import (
	"fmt"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Native window ownership is deliberately small: no graphics context is
// created here. WebGPU receives the HWND and owns all presentation.
var winUser = windows.NewLazySystemDLL("user32.dll")

// Resolve each symbol lazily once, not once per high-frequency input event.
var nativeUserProcs = func() map[string]*windows.LazyProc {
	result := make(map[string]*windows.LazyProc)
	for _, name := range []string{"DefWindowProcW", "SetProcessDpiAwarenessContext", "LoadCursorW", "RegisterClassExW", "AdjustWindowRectEx", "CreateWindowExW", "RegisterRawInputDevices", "ShowWindow", "DestroyWindow", "SetWindowPos", "ClientToScreen", "SetCursorPos", "GetClientRect", "MapWindowPoints", "ClipCursor", "SetCursor", "PeekMessageW", "TranslateMessage", "DispatchMessageW", "SetCapture", "ReleaseCapture", "GetRawInputData"} {
		result[name] = winUser.NewProc(name)
	}
	return result
}()

func nativeUserProc(name string) *windows.LazyProc { return nativeUserProcs[name] }

var winKernel = windows.NewLazySystemDLL("kernel32.dll")
var winDef = nativeUserProc("DefWindowProcW")
var winWindows = map[uintptr]*nativeWindow{}
var winCallback = syscall.NewCallback(nativeWindowProc)
var winClassName, _ = windows.UTF16PtrFromString("GoCraftWebGPUWindow")

type winPoint struct{ X, Y int32 }
type winRect struct{ Left, Top, Right, Bottom int32 }
type winClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	Menu, Name                         *uint16
	SmallIcon                          uintptr
}
type winMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          winPoint
	Private        uint32
}
type rawMousePacket struct {
	Kind, Size          uint32
	Device, WParam      uintptr
	Flags               uint16
	Padding             uint16
	Buttons, RawButtons uint32
	X, Y                int32
	Extra               uint32
}
type nativeWindow struct {
	hwnd                                 uintptr
	width, height                        int
	pendingWidth, pendingHeight          int
	closed, focused, minimized, captured bool
	started                              time.Time
	highSurrogate                        uint16
	frame                                windowInputFrame
}

func createNativeWindow(width, height int, visible bool) (*nativeWindow, error) {
	if p := nativeUserProc("SetProcessDpiAwarenessContext"); p.Find() == nil {
		p.Call(^uintptr(3))
	}
	instance, _, _ := winKernel.NewProc("GetModuleHandleW").Call(0)
	cursor, _, _ := nativeUserProc("LoadCursorW").Call(0, 32512)
	class := winClass{Proc: winCallback, Instance: instance, Cursor: cursor, Name: winClassName}
	class.Size = uint32(unsafe.Sizeof(class))
	if ok, _, err := nativeUserProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class))); ok == 0 && err != syscall.Errno(1410) {
		return nil, fmt.Errorf("register window class: %w", err)
	}
	r := winRect{Right: int32(width), Bottom: int32(height)}
	const style = 0x00cf0000 // overlapped, resizeable window
	nativeUserProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&r)), style, 0, 0)
	title, _ := windows.UTF16PtrFromString("GoCraft — WebGPU")
	hwnd, _, err := nativeUserProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(winClassName)), uintptr(unsafe.Pointer(title)), style, 0x80000000, 0x80000000, uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0, 0, instance, 0)
	if hwnd == 0 {
		return nil, fmt.Errorf("create window: %w", err)
	}
	w := &nativeWindow{hwnd: hwnd, width: width, height: height, started: time.Now()}
	winWindows[hwnd] = w
	// Foreground-only relative mouse input; do not intercept other apps' input.
	device := struct {
		Page, Usage uint16
		Flags       uint32
		Target      uintptr
	}{1, 2, 0, hwnd}
	if ok, _, err := nativeUserProc("RegisterRawInputDevices").Call(uintptr(unsafe.Pointer(&device)), 1, unsafe.Sizeof(device)); ok == 0 {
		w.Close()
		return nil, fmt.Errorf("register mouse: %w", err)
	}
	if visible {
		nativeUserProc("ShowWindow").Call(hwnd, 5)
	}
	return w, nil
}

func (w *nativeWindow) Close() {
	if w.hwnd == 0 {
		return
	}
	w.SetCaptured(false)
	nativeUserProc("DestroyWindow").Call(w.hwnd)
	delete(winWindows, w.hwnd)
	w.hwnd = 0
}
func (w *nativeWindow) Resize(width, height int) {
	if width > 0 && height > 0 {
		w.pendingWidth, w.pendingHeight = width, height
	}
}

// Apply settings before capturing a new frame, not during menu drawing.
func (w *nativeWindow) applyPendingResize() {
	width, height := w.pendingWidth, w.pendingHeight
	w.pendingWidth, w.pendingHeight = 0, 0
	if width <= 0 || height <= 0 || (width == w.width && height == w.height) {
		return
	}
	r := winRect{Right: int32(width), Bottom: int32(height)}
	nativeUserProc("AdjustWindowRectEx").Call(uintptr(unsafe.Pointer(&r)), 0x00cf0000, 0, 0)
	nativeUserProc("SetWindowPos").Call(w.hwnd, 0, 0, 0, uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x0016)
}
func (w *nativeWindow) SetPosition(x, y int) {
	p := winPoint{int32(x), int32(y)}
	nativeUserProc("ClientToScreen").Call(w.hwnd, uintptr(unsafe.Pointer(&p)))
	nativeUserProc("SetCursorPos").Call(uintptr(p.X), uintptr(p.Y))
	w.frame.Mouse = uiPoint{float32(x), float32(y)}
	w.frame.Delta = uiPoint{}
}
func (w *nativeWindow) SetCaptured(captured bool) {
	w.captured = captured
	w.updateCapture()
	w.frame.Delta = uiPoint{}
}
func (w *nativeWindow) updateCapture() {
	if w.captured && w.focused && !w.minimized {
		r := winRect{}
		nativeUserProc("GetClientRect").Call(w.hwnd, uintptr(unsafe.Pointer(&r)))
		nativeUserProc("MapWindowPoints").Call(w.hwnd, 0, uintptr(unsafe.Pointer(&r)), 2)
		nativeUserProc("ClipCursor").Call(uintptr(unsafe.Pointer(&r)))
		nativeUserProc("SetCursor").Call(0)
	} else {
		nativeUserProc("ClipCursor").Call(0)
		cursor, _, _ := nativeUserProc("LoadCursorW").Call(0, 32512)
		nativeUserProc("SetCursor").Call(cursor)
	}
}
func (w *nativeWindow) Poll() {
	w.applyPendingResize()
	w.frame.Pressed = [keyCount]bool{}
	w.frame.MousePressed = [mouseButtonCount]bool{}
	w.frame.MouseReleased = [mouseButtonCount]bool{}
	w.frame.Delta = uiPoint{}
	w.frame.Wheel = 0
	w.frame.Text = w.frame.Text[:0]
	w.frame.textIndex = 0
	var msg winMessage
	for {
		ok, _, _ := nativeUserProc("PeekMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0, 1)
		if ok == 0 {
			break
		}
		if msg.Message == 0x12 {
			w.closed = true
			continue
		}
		nativeUserProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
		nativeUserProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
	}
	w.frame.Width, w.frame.Height = w.width, w.height
	w.frame.Time = time.Since(w.started).Seconds()
	windowFrame = w.frame
	windowCursor = w
}
func nativeKey(code, flags uintptr) int32 {
	if code >= 0x31 && code <= 0x39 {
		return keyOne + int32(code-0x31)
	}
	switch code {
	case 'A':
		return keyA
	case 'D':
		return keyD
	case 'E':
		return keyE
	case 'Q':
		return keyQ
	case 'R':
		return keyR
	case 'S':
		return keyS
	case 'W':
		return keyW
	case 0x20:
		return keySpace
	case 0x1b:
		return keyEscape
	case 0x0d:
		return keyEnter
	case 0x08:
		return keyBackspace
	case 0x70:
		return keyF1
	case 0x72:
		return keyF3
	case 0x10:
		if (flags>>16)&255 == 0x36 {
			return keyRightShift
		}
		return keyLeftShift
	case 0x11:
		if flags&(1<<24) != 0 {
			return keyRightControl
		}
		return keyLeftControl
	}
	return -1
}
func nativeWindowProc(hwnd uintptr, message uint32, wp, lp uintptr) uintptr {
	w := winWindows[hwnd]
	if w != nil {
		switch message {
		case 0x10:
			w.closed = true
			return 0 // WM_CLOSE: renderer tears down before HWND
		case 0x5:
			w.minimized = wp == 1
			w.width = int(lp & 65535)
			w.height = int((lp >> 16) & 65535)
			w.updateCapture()
		case 0x3:
			w.updateCapture()
		case 0x7:
			w.focused = true
			w.frame.Delta = uiPoint{}
			w.updateCapture()
		case 0x8:
			w.focused = false
			w.frame.Down = [keyCount]bool{}
			w.frame.Pressed = [keyCount]bool{}
			w.frame.MouseDown = [mouseButtonCount]bool{}
			w.frame.MousePressed = [mouseButtonCount]bool{}
			w.frame.MouseReleased = [mouseButtonCount]bool{true, true}
			w.frame.Delta = uiPoint{}
			w.highSurrogate = 0
			w.updateCapture()
		case 0x20:
			if w.captured && w.focused && lp&65535 == 1 {
				nativeUserProc("SetCursor").Call(0)
				return 1
			}
		case 0x100, 0x104, 0x101, 0x105:
			if k := nativeKey(wp, lp); k >= 0 {
				down := message == 0x100 || message == 0x104
				if down && !w.frame.Down[k] {
					w.frame.Pressed[k] = true
				}
				w.frame.Down[k] = down
			}
		case 0x102:
			c := uint16(wp)
			if c >= 0xd800 && c <= 0xdbff {
				w.highSurrogate = c
			} else if c >= 0xdc00 && c <= 0xdfff && w.highSurrogate != 0 {
				w.frame.Text = append(w.frame.Text, utf16.DecodeRune(rune(w.highSurrogate), rune(c)))
				w.highSurrogate = 0
			} else {
				w.highSurrogate = 0
				if c != 0 {
					w.frame.Text = append(w.frame.Text, rune(c))
				}
			}
		case 0x200:
			w.frame.Mouse = uiPoint{float32(int16(lp & 65535)), float32(int16((lp >> 16) & 65535))}
		case 0x201, 0x202, 0x204, 0x205:
			b := mouseLeft
			if message == 0x204 || message == 0x205 {
				b = mouseRight
			}
			down := message == 0x201 || message == 0x204
			w.frame.MouseDown[b] = down
			if down {
				w.frame.MousePressed[b] = true
				nativeUserProc("SetCapture").Call(hwnd)
			} else {
				w.frame.MouseReleased[b] = true
				if !w.frame.MouseDown[0] && !w.frame.MouseDown[1] {
					nativeUserProc("ReleaseCapture").Call()
				}
			}
		case 0x20a:
			w.frame.Wheel += float32(int16((wp>>16)&65535)) / 120
		case 0xff:
			if w.captured && w.focused {
				var packet rawMousePacket
				size := uint32(unsafe.Sizeof(packet))
				header := uintptr(8 + 2*unsafe.Sizeof(uintptr(0)))
				read, _, _ := nativeUserProc("GetRawInputData").Call(lp, 0x10000003, uintptr(unsafe.Pointer(&packet)), uintptr(unsafe.Pointer(&size)), header)
				if read != uintptr(^uint32(0)) && read >= header+20 && packet.Kind == 0 && packet.Flags&1 == 0 {
					w.frame.Delta.X += float32(packet.X)
					w.frame.Delta.Y += float32(packet.Y)
				}
			}
		case 0x2e0: // WM_DPICHANGED: accept the suggested physical bounds
			var r winRect
			winKernel.NewProc("RtlMoveMemory").Call(uintptr(unsafe.Pointer(&r)), lp, unsafe.Sizeof(r))
			nativeUserProc("SetWindowPos").Call(hwnd, 0, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right-r.Left), uintptr(r.Bottom-r.Top), 0x14)
		}
	}
	result, _, _ := winDef.Call(hwnd, uintptr(message), wp, lp)
	return result
}

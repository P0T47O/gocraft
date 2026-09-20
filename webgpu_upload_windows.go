//go:build windows

package main

import (
	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// Each draw in a frame owns a distinct destination. Queue writes happen before
// the submitted render pass, not between its draw commands. Slots are reused
// next frame, where queue ordering protects the preceding frame's reads.
type webGPUFrameUploads struct {
	slots []*wgpu.Buffer
	next  int
}

func (u *webGPUFrameUploads) begin() { u.next = 0 }

func (u *webGPUFrameUploads) write(device *wgpu.Device, queue *wgpu.Queue, data []byte, usage gputypes.BufferUsage) (*wgpu.Buffer, error) {
	if u.next == len(u.slots) {
		u.slots = append(u.slots, nil)
	}
	b := u.slots[u.next]
	if b == nil || b.Size() < uint64(len(data)) {
		size := uint64(256)
		for size < uint64(len(data)) {
			size *= 2
		}
		next, err := device.CreateBuffer(&wgpu.BufferDescriptor{Label: "GoCraft frame upload", Size: size, Usage: usage | gputypes.BufferUsageCopyDst})
		if err != nil {
			return nil, err
		}
		if b != nil {
			b.Release()
		}
		b = next
		u.slots[u.next] = b
	}
	if err := queue.WriteBuffer(b, 0, data); err != nil {
		return nil, err
	}
	u.next++
	return b, nil
}

func (u *webGPUFrameUploads) close() {
	for _, b := range u.slots {
		if b != nil {
			b.Release()
		}
	}
	u.slots = nil
	u.next = 0
}

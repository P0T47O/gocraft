# WebGPU feature index

See [CODE_INDEX.md](CODE_INDEX.md) for the maintained full feature map and [CODE_SYMBOLS.md](CODE_SYMBOLS.md) for symbols.

| Area | Files |
| --- | --- |
| Native Win32 window, raw mouse, focus, resize and UTF-16 input | window_win32_windows.go |
| Independent WebGPU main loop and session lifecycle | native_game_windows.go, native_window_state.go, game_session.go |
| Window-neutral input, camera, menu state and layouts | window_input.go, game_camera.go, menu_screens.go, ui_menu.go, inventory_layout.go, ui.go |
| Menu draw contract and WebGPU ordered batches/scissors | menu_painter.go, menu_webgpu_windows.go, menu_death.go, ui_palette.go |
| Surface, world pipelines and camera | webgpu_game_windows.go, webgpu_world_renderer_windows.go, webgpu_camera_windows.go |
| One-pass world/entities/HUD/inventory/menu compositor | webgpu_gameplay_windows.go |
| Atlas, mipmaps, animation and AF | webgpu_atlas.go, webgpu_mipmap.go, webgpu_animation_windows.go, webgpu_filter_windows.go |
| Compact GPU mesh payload and upload slots | platform/compact_vertex.go, platform/webgpu_backend.go, webgpu_upload_windows.go |
| Entities, conservative culling and split batches | webgpu_entities_windows.go, webgpu_entity_bounds_windows.go |
| HUD/text/inventory/container | webgpu_hud_windows.go, webgpu_text_windows.go, webgpu_inventory_windows.go, webgpu_container_windows.go |
| GPU regressions and native-window/session integration | webgpu_regression_windows_test.go, webgpu_mip_regression_windows_test.go, native_window_windows_test.go |
| Optional GPU menu PNG readback | menu_preview_webgpu_windows_test.go |

## Current state

On Windows, `go run . -webgpu` creates a native Win32 window and uses WebGPU for menus and gameplay. No Raylib window/input/presentation is used on that path. Session exit releases world meshes but retains the menu window and GPU atlas; final shutdown releases GPU before destroying the HWND.

WebGPU is the default. The explicit `-webgpu=false` legacy path, legacy assets, OpenGL renderer, older UI/mob previews and standalone smoke tool still compile against Raylib. This is **not** complete dependency removal. WebGPU's built-in font is ASCII-only. IME composition and live cross-monitor DPI behavior remain unverified.

Validation: `go test ./...`, `go vet ./...`, `go build .`, and `go run ./tools/codeindex -check`. On Windows, set `GOCRAFT_WEBGPU_REGRESSION=1` and run `go test . -run 'TestNativeWebGPUWindow|TestWebGPUUIUploadIsolationGPU|TestWebGPUMipStabilityGPU' -count=1`. The native test uses a hidden window and temporary save, not user worlds. Optional `GOCRAFT_NATIVE_MENU_PREVIEW` names a PNG output path.
The five WebGPU terrain previews now use native windows (helper: `webgpu_preview_window_windows_test.go`). Set their existing opt-in environment flag plus `GOCRAFT_WEBGPU_PREVIEW_FRAMES=3` for hidden, bounded execution with a resize. `mob_pose.go` holds the active CPU bone-pose calculation, shared by WebGPU rendering and normal unit tests.

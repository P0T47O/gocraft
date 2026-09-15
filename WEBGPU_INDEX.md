# WebGPU experiment code index

This supplements `CODE_INDEX.md` without replacing the main project's complete navigation.

| Function | File / entry |
| --- | --- |
| Migration plan and milestones | `WEBGPU_MIGRATION.md` |
| Backend-neutral vertex and current OpenGL mesh | `platform/mesh.go` |
| `MeshBackend` / `MeshHandle`, upload/draw routing | `platform/renderer.go` |
| Real WebGPU vertex/index buffer backend | `platform/webgpu_backend.go`: `NewWebGPUMeshBackend`, `UploadChecked`, `DrawPass` |
| Windows WebGPU surface + indexed mesh smoke probe | `cmd/webgpu-smoke/main_windows.go` |
| Chunk mesh backend-neutral GPU handle | `render_mesh.go` |
| CPU-built WebGPU texture atlas | `webgpu_atlas.go` |
| Live WebGPU world renderer: camera, depth, opaque/cutout, water/glass, fog, visible sections | `webgpu_world_renderer_windows.go` |
| Live WebGPU shape/icon HUD: crosshair, hotbar, vitals | `webgpu_hud_windows.go` |
| Live WebGPU text overlay: built-in pixel font, hotbar labels/counts, chat, debug, pause/death text | `webgpu_text_windows.go` |
| Single-pass live gameplay compositor | `webgpu_gameplay_windows.go`: `DrawGameplay` |
| `-webgpu` Playing-state lifetime and renderer handoff | `webgpu_game_windows.go` |

Current state: on real Windows hardware the Raylib-created HWND is presented by WebGPU through the GoGPU Vulkan backend. The live `-webgpu` path now renders streamed chunk terrain, atlas textures, cutout foliage, transparent water/glass, fog, crosshair/hotbar/vitals, and an ASCII text overlay in WebGPU while Raylib/OpenGL presentation stays idle during `StatePlaying`.

The text overlay currently covers selected-item labels and stack counts, survival status text, chat input/history, debug metrics, pause text, and a keyboard-respawn death prompt. Inventory/container UI, full pause-menu controls, entities, mining cracks, item/tool icons, animated/special materials, and the final window/input/type decoupling are still pending.

The target architecture remains zero-Raylib on the WebGPU branch. Raylib is temporarily retained for the native window/input owner, camera/vector/rectangle types, menus, the OpenGL reference renderer, and several shared gameplay/server type leaks. Remove those incrementally after WebGPU feature parity rather than as one flag-day rewrite.

Local validation for the live path: `go test ./...`, `go vet ./...`, `go build .`, then `go run . -webgpu`. Preview tests are opt-in only and are no longer part of the normal iteration loop. After new Go symbols stabilize, run `go run ./tools/codeindex -write` and `go run ./tools/codeindex -check` before considering the branch clean.

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
| CPU-built WebGPU texture atlas, including block/item sprites, mob skins and crack stages | `webgpu_atlas.go` |
| Live WebGPU world renderer: camera, depth, opaque/cutout, water/glass, fog, visible sections | `webgpu_world_renderer_windows.go` |
| Live WebGPU entity/effect batch: remote players, dropped items, content-driven animated mobs, mining cracks | `webgpu_entities_windows.go` |
| Live WebGPU shape/icon HUD: crosshair, hotbar, vitals | `webgpu_hud_windows.go` |
| Live WebGPU text overlay: built-in pixel font, hotbar labels/counts, chat, debug, pause/death text | `webgpu_text_windows.go` |
| WebGPU creative/survival inventory and item/tool presentation | `webgpu_inventory_windows.go` |
| WebGPU chest/furnace container presentation | `webgpu_container_windows.go` |
| Single-pass live gameplay compositor | `webgpu_gameplay_windows.go`: `DrawGameplay` |
| `-webgpu` Playing-state lifetime and renderer handoff | `webgpu_game_windows.go` |

Current state: on real Windows hardware the Raylib-created HWND is presented by WebGPU through the GoGPU Vulkan backend. The live `-webgpu` path renders streamed chunk terrain, atlas textures, cutout foliage, transparent water/glass, fog, remote players, dropped block/tool items, content-driven mobs, mining crack overlays, crosshair/hotbar/vitals, ASCII text, creative/survival inventories, standalone item/tool sprites, and chest/furnace container screens while Raylib/OpenGL presentation stays idle during `StatePlaying`.

Entity presentation uses one dynamic WebGPU vertex/index batch in world space. Mob geometry now follows the JSON bone model and animation channels already used by the reference renderer, including walk/flee leg swing, hurt tint and death tilt. Larger mob skins are nearest-neighbor reduced into a single atlas cell while preserving normalized sub-UV addressing. Mining cracks reuse the same batch as a slightly expanded textured cube to avoid z-fighting.

Container and inventory input still reuse the existing Raylib-driven hit testing and authoritative networking logic; this stage migrates presentation first. The target architecture remains zero-Raylib on the WebGPU branch. Raylib is temporarily retained for the native window/input owner, camera/vector/rectangle types, menus, the OpenGL reference renderer, and several shared gameplay/server type leaks. First-person held-item rendering, animated/special materials, Unicode text, full pause controls, and final window/input/type decoupling are still pending.

Local validation for the live path: `go test ./...`, `go vet ./...`, `go build .`, then `go run . -webgpu`. Preview tests are opt-in only and are no longer part of the normal iteration loop. After new Go symbols stabilize, run `go run ./tools/codeindex -write` and `go run ./tools/codeindex -check` before considering the branch clean.

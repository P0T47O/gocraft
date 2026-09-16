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
| Backend-neutral atlas UV rectangle shared by legacy/WebGPU mesh consumers | `render_assets.go`: `AtlasRect` |
| CPU-built WebGPU texture atlas, including block/item sprites, mob skins and crack stages | `webgpu_atlas.go` |
| Native WebGPU camera matrices, scene uniforms and visible-section collection | `webgpu_camera_windows.go` |
| Live WebGPU world renderer: surface, depth, opaque/cutout, water/glass, fog, mesh drawing | `webgpu_world_renderer_windows.go` |
| Live WebGPU entity/effect batch: remote players, dropped items, content-driven animated mobs, mining cracks | `webgpu_entities_windows.go` |
| Live WebGPU shape/icon HUD: crosshair, hotbar, vitals | `webgpu_hud_windows.go` |
| Live WebGPU text overlay: built-in pixel font, hotbar labels/counts, chat, debug, pause/death text | `webgpu_text_windows.go` |
| WebGPU creative/survival inventory and item/tool presentation | `webgpu_inventory_windows.go` |
| WebGPU chest/furnace container presentation | `webgpu_container_windows.go` |
| Single-pass live gameplay compositor | `webgpu_gameplay_windows.go`: `DrawGameplay` |
| `-webgpu` Playing-state lifetime, frame snapshot and temporary Raylib window/input bridge | `webgpu_game_windows.go` |

Current state: on real Windows hardware the Raylib-created HWND is presented by WebGPU through the GoGPU Vulkan backend. The live `-webgpu` path renders streamed chunk terrain, atlas textures, cutout foliage, transparent water/glass, fog, remote players, dropped block/tool items, content-driven mobs, mining crack overlays, crosshair/hotbar/vitals, ASCII text, creative/survival inventories, standalone item/tool sprites, and chest/furnace container screens while Raylib/OpenGL presentation stays idle during `StatePlaying`.

Entity presentation uses one dynamic WebGPU vertex/index batch in world space. Mob geometry follows the JSON bone model and animation channels already used by the reference renderer, including walk/flee leg swing, hurt tint and death tilt. Larger mob skins are stored in the atlas at their native resolution within the padded cell. Mining cracks reuse the same batch as a slightly expanded textured cube to avoid z-fighting.

No new gameplay presentation is being added during the dependency-removal phase. The experimental viewmodel prototype was removed because the reference game did not have that feature. HUD, text, inventory, container, entity/effect presentation, gameplay composition, native camera math, the CPU WebGPU atlas builder and the core WebGPU world renderer no longer import Raylib directly. `webgpu_game_windows.go` is the intentional temporary presentation boundary that snapshots screen size, time, camera and mouse data and supplies the Raylib-created HWND used to create the WebGPU surface. The obsolete standalone world `Draw` path and duplicate Raylib-camera scene/culling helpers have been removed.

Milestone A (runtime WebGPU renderer internals are Raylib-free) is complete, pending normal local compile/runtime validation. `TextureAtlas.UVs` now uses backend-neutral `AtlasRect`, so the WebGPU atlas no longer inherits `rl.Rectangle` from the legacy mesher. Raylib remains for the native window/input owner, menus, the OpenGL reference renderer and several shared gameplay types. Milestone B will remove those gameplay/network/world type leaks incrementally, starting with low-risk boundaries such as `Client.Update(*rl.Camera3D)`, before replacing window/input. The OpenGL/Raylib renderer stays as the reference path until existing feature parity is stable.

Local validation for the live path: `go test ./...`, `go vet ./...`, `go build .`, then `go run . -webgpu`. Preview tests are opt-in only and are no longer part of the normal iteration loop. After new Go symbols stabilize, run `go run ./tools/codeindex -write` and `go run ./tools/codeindex -check` before considering the branch clean.

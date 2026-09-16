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
| CPU-built WebGPU texture atlas, including block/item sprites, mob skins, crack stages and retained animated-liquid frames | `webgpu_atlas.go` |
| Atlas-backed water/lava frame uploads using existing `.mcmeta` timing | `webgpu_animation_windows.go`: `updateWebGPUAtlasAnimations` |
| Native WebGPU camera matrices, scene uniforms and visible-section collection | `webgpu_camera_windows.go` |
| Live WebGPU world renderer: surface, depth, opaque/cutout, water/glass, fog, mesh drawing | `webgpu_world_renderer_windows.go` |
| Live WebGPU entity/effect batch: remote players, dropped items, content-driven animated mobs, mining cracks | `webgpu_entities_windows.go` |
| Live WebGPU shape/icon HUD: crosshair, hotbar, vitals | `webgpu_hud_windows.go` |
| Live WebGPU text overlay: built-in pixel font, hotbar labels/counts, chat, debug, pause/death text | `webgpu_text_windows.go` |
| WebGPU creative/survival inventory and item/tool presentation | `webgpu_inventory_windows.go` |
| WebGPU chest/furnace container presentation | `webgpu_container_windows.go` |
| Single-pass live gameplay compositor | `webgpu_gameplay_windows.go`: `DrawGameplay` |
| `-webgpu` Playing-state lifetime, frame snapshot and temporary Raylib window/input bridge | `webgpu_game_windows.go` |
| Raylib-free TCP client transport and movement snapshot emission | `client.go`: `Client.Update` |
| Backend-neutral voxel DDA ray cast | `world_ray.go`: `rayCast` |
| Temporary Raylib ray adapter for the current input path | `world_ray_raylib.go`: `HitTest` |
| Renderer-neutral gameplay vectors and collision/movement core | `game_math.go`, `actor_physics.go`, `player_movement.go` |
| Temporary Raylib vector adapters for camera/input and legacy tests | `input_physics_raylib.go` |
| Raylib-free mob movement/spawn simulation and authoritative attack validation | `mob_simulation.go`, `mob_server.go`, `mob_protocol.go` |
| Reference-renderer-only mob debug AABB | `mob_render_bounds.go` |

Current state: on real Windows hardware the Raylib-created HWND is presented by WebGPU through the GoGPU Vulkan backend. The live `-webgpu` path renders streamed chunk terrain, atlas textures, animated water/lava atlas tiles, cutout foliage, transparent water/glass, fog, remote players, dropped block/tool items, content-driven mobs, mining crack overlays, crosshair/hotbar/vitals, ASCII text, creative/survival inventories, standalone item/tool sprites, and chest/furnace container screens while Raylib/OpenGL presentation stays idle during `StatePlaying`.

Entity presentation uses one dynamic WebGPU vertex/index batch in world space. Mob geometry follows the JSON bone model and animation channels already used by the reference renderer, including walk/flee leg swing, hurt tint and death tilt. Larger mob skins are stored in the atlas at their native resolution within the padded cell. Mining cracks reuse the same batch as a slightly expanded textured cube to avoid z-fighting.

Animated block textures keep the same vertical-strip interpretation and `.mcmeta` `frametime` used by the reference renderer. The WebGPU atlas retains the decoded frames on the CPU and updates only the affected atlas tile when the frame index changes; meshes and UVs remain stable. This restores existing water/lava animation parity rather than adding a new gameplay feature.

No new gameplay presentation is being added during the dependency-removal phase. The experimental viewmodel prototype was removed because the reference game did not have that feature. HUD, text, inventory, container, entity/effect presentation, gameplay composition, native camera math, the CPU WebGPU atlas builder and the core WebGPU world renderer no longer import Raylib directly. `webgpu_game_windows.go` is the intentional temporary presentation boundary that snapshots screen size, time, camera and mouse data and supplies the Raylib-created HWND used to create the WebGPU surface. The obsolete standalone world `Draw` path and duplicate Raylib-camera scene/culling helpers have been removed.

Milestone A (runtime WebGPU renderer internals are Raylib-free) is complete, pending normal local compile/runtime validation. `TextureAtlas.UVs` now uses backend-neutral `AtlasRect`, so the WebGPU atlas no longer inherits `rl.Rectangle` from the legacy mesher. Milestone B is underway: `Client.Update` accepts primitive movement state; `world_ray.go` performs the voxel DDA with primitive coordinates/directions and a backend-neutral hit normal; authoritative mob attack validation performs its own slab AABB test; and shared actor/player/mob movement now uses `gameVec3` instead of `rl.Vector3`. `actor_physics.go`, `player_movement.go`, `mob_simulation.go`, and `mob_server.go` no longer import Raylib.

Raylib vector compatibility for the existing camera/input path is now concentrated in `input_physics_raylib.go`, while the reference mob debug bounds live in `mob_render_bounds.go`. Remaining gameplay leaks include `player_vitals.go` and the input-side `rl.Ray`/camera APIs. After those are narrowed, the next major boundary is the native window/input owner; the OpenGL/Raylib renderer remains only as the temporary reference path until parity is stable.

Local validation for the live path: `go test ./...`, `go vet ./...`, `go build .`, then `go run . -webgpu`. Preview tests are opt-in only and are no longer part of the normal iteration loop. This physics batch adds, deletes and moves Go symbols, so regenerate the symbol index after validation: `go run ./tools/codeindex -write` then `go run ./tools/codeindex -check`.

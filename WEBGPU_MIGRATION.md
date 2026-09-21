# WebGPU renderer experiment

## Resolution-change recovery

Reproduced `hal: surface outdated` by applying a resolution change after input capture and before drawing the paused world. Native settings resize now coalesces until the next event poll. Both menu and gameplay classify only `wgpu.ErrSurfaceOutdated` (including wrapped errors) as recoverable, discard any stale acquisition, skip that frame and reconfigure on the next draw even at unchanged dimensions. Suboptimal acquisitions also request reconfiguration. Device loss, surface loss, allocation errors and unrelated errors still propagate. UI actions are not replayed during recovery.

The hidden-window regression now exercises same-frame settings changes across four resolutions in each of two temporary-save sessions, same-size recovery, and an external resize after a menu snapshot. The original regression failed before this fix and passed afterward; user settings and worlds are not modified by it.

## Native window and full menu presentation (2026-09-21)

`-webgpu` now creates an independent Win32 window before any Raylib initialization. Native input handles relative raw mouse movement, keyboard/text events, focus loss, minimization, resizing and DPI messages. Shared menus emit ordered rectangle/text batches through a neutral painter; WebGPU presents main/world/create/multiplayer/settings pages and pause/death overlays. The native path reports initialization errors rather than opening an implicit OpenGL fallback. World exit retains the native window, device and atlas; final shutdown closes gameplay before GPU and HWND.

A hidden-window GPU integration test validates all menu pages, resize, native key/UTF-16 handling, focus release, and two temporary-save sessions including pause/settings and return to menu. Main-menu pixels were read back and visually inspected. Real cross-monitor dragging, physical mouse motion and IME composition are not manually verified. The built-in font still only renders ASCII.

Raylib remains a build dependency through the retained non-WebGPU path, resource types and legacy tools/tests. Older sections below record migration history and do not describe the current window owner.


## Optimization and dependency cleanup (2026-09-20)

### Follow-up: texture sharpness and mip disable

The Vulkan HAL in the pinned dependency interprets sampler `LodMaxClamp=0` as unlimited. Mipmap-off and HUD now bind a view exposing only mip zero; the world retains a full-chain view for mipmap-on. A scalar clamp alone is insufficient.

AF requires linear sampler filters, but magnified pixel art now uses explicit base-level texel loads when its screen-space footprint is at most one texel. Minification retains the anisotropic/mipmap sampler. Derivatives and implicit-LOD sampling are evaluated before the varying branch.

`GOCRAFT_WEBGPU_REGRESSION=1 go test . -run TestWebGPUMipStabilityGPU -v -count=1` verifies eight translated textured frames with updates to a neighboring animated atlas cell, then verifies actual mip-on/off/on selection using color-coded mip levels. On RTX 5060/Vulkan, the near checker range improved from 56–199 in the initial frame to 0–255 and remained 0–255 throughout the movement sequence. The mip toggle reads blue/red/blue as expected. This synthetic test does **not** fully reproduce the reported whole-scene sudden blur while walking.

- Live world/entity vertices are 24 bytes (position, UV, RGBA); diagnostic backends keep the original normal-bearing 36-byte payload. This reduces vertex bytes by one third, not total VRAM by one third.
- HUD/text/entity draws use persistent per-frame upload slots. Separate draws never overwrite the same destination before the frame submission; slots grow geometrically and are reused next frame.
- World atlas has mip levels 0–4. Settings apply mipmap and AF 1/2/4/8/16 through sampler/bind-group replacement. AF uses linear min/mag/mip filtering as required by WebGPU, with magnification handled by base texel loads; 16x restricts max LOD to 3 for atlas padding. Disabling mipmaps binds a mip-zero-only view. HUD uses a separate nearest sampler and base-only view.
- Animated atlas updates refresh the entire extruded cell and every mip, using reusable scratch storage. RGB downsampling is alpha-weighted to avoid transparent-black fringes.
- Entities are culled using conservative rotation/animation-independent bounds and render distance, then CPU batches are reused and split instead of failing at the old fixed capacity. Models still animate on CPU; instancing/skinning is not implemented.
- Solid world geometry uses backface culling; cutout vegetation, entities and translucent surfaces retain double-sided pipelines. Six-face winding and the compact pipeline are regression-tested.
- WebGPU sessions no longer initialize legacy game textures, models, icons, shaders or materials. Initialization happens before workers start; failure falls back to OpenGL resource loading. Raylib remains the window/input/menu owner.
- CPU chunk meshing, mesh lighting/tint/math, shared mesh handles, filtering policy, frame snapshots and settings persistence no longer directly import Raylib. Legacy upload/window adaptation remains in explicitly separated files.

Validation: full Go tests, vet, build and code-index checks. Opt-in `GOCRAFT_WEBGPU_REGRESSION=1 go test . -run TestWebGPUUIUploadIsolationGPU -v -count=1` runs real **offscreen** GPU checks: two UI draws remain red/green across frame reuse, every filtering setting creates successfully, a compact blue triangle survives backface culling, and 1500 synthetic players span multiple entity batches. Observed adapter: RTX 5060 / Vulkan. This is not a full interactive gameplay or frame-rate benchmark.

Remaining performance work: measure chunk buffer allocation/driver submission costs before introducing a bounded buffer pool, shared terrain arenas or indirect drawing. No shared terrain allocator or GPU-driven renderer is introduced in this change.

The sections below record the earlier migration milestones.

This branch is an isolated experiment for evaluating a WebGPU rendering backend without destabilizing `main`.

## Current status

The Raylib/OpenGL reference renderer still builds and renders the existing GoCraft world after the backend-neutral mesh refactor.

The Windows experiment has now been validated through progressively larger milestones on real hardware (NVIDIA GeForce RTX 4070, Vulkan backend):

- WebGPU presentation into the existing Raylib-created HWND.
- Real vertex/index buffers using GoCraft's existing 36-byte `platform.Vertex` layout.
- Perspective camera and `Depth24Plus` depth testing.
- Real procedural world generation and the existing chunk mesher.
- A 5x5 neighbor halo feeding a rendered inner 3x3 terrain region.
- A CPU-built block texture atlas sampled through WebGPU using the existing chunk-mesher UV decisions, including vertex AO/tint/light modulation and cutout alpha discard.
- Water/glass alpha blending with depth testing, depth writes disabled, back-to-front transparent batch ordering and shared distance fog.

The experiment uses `github.com/gogpu/wgpu v0.34.5` with the pure-Go native backend. The earlier `go-webgpu/webgpu` + `wgpu-native` path was abandoned after its v29 vertex-attribute ABI gap caused native pipeline panics. `wgpu_native.dll` is no longer required for this branch's normal WebGPU path.

The validated preview pipeline has now been extracted into `webgpu_world_renderer_windows.go`. Running the game with `-webgpu` keeps Raylib as the window/input owner and uses the normal OpenGL menu, then switches the Playing state to WebGPU presentation. The live WebGPU world renderer reuses the existing visible-section/frustum logic, submits normal mesh jobs, receives live chunk uploads through `WebGPUMeshBackend`, draws opaque/cutout then sorted water/glass, and presents directly to the Raylib-created HWND.

This is intentionally not parity-complete yet: entities, HUD/inventory/pause overlays, mining crack, animated/special non-atlas materials and some underwater presentation remain on the follow-up list. The `-webgpu` path is therefore an integration test, not yet the default renderer.

## Initial audit

GoCraft already owns explicit OpenGL chunk mesh upload/draw state (`platform/mesh.go`, `render_mesh.go`) instead of relying only on Raylib high-level drawing. CPU-side meshing can therefore survive the migration, but WebGPU is a real backend rewrite rather than a library-name substitution.

Raylib also appears in shared/non-rendering structures (client state, world ray queries, saves, mob server and common types). Those dependencies should be removed incrementally rather than with a flag-day conversion.

## Strategy

1. Keep `main` untouched on the default path; all experimental behavior remains opt-in with `-webgpu` on `experiment/webgpu` until parity and benchmarking justify a default switch.
2. Preserve the CPU mesh payload (`Position`, `Texcoord`, byte `Color`, `Normal`) and indices while making GPU handles backend-owned.
3. Temporarily keep Raylib/OpenGL as the reference path while WebGPU reaches useful parity. Two permanent renderers are not a goal.
4. Introduce narrow seams for mesh upload/draw/unload, textures, pipelines, frame/surface management and camera uniforms. Avoid a giant generic graphics API.
5. Keep all GPU creation/draw/destruction on the render thread.

## Milestones

- [x] Backend-neutral mesh payload/handle boundary; existing renderer remains functional.
- [x] Select and pin the Go WebGPU binding/backend (`gogpu/wgpu v0.34.5`, pure-Go native backend).
- [x] Prove WebGPU instance/device/surface presentation on the current Raylib-created HWND.
- [x] Replace staged CPU payloads with real WebGPU vertex/index GPU buffers.
- [x] Prove a WGSL pipeline matching the existing 36-byte vertex semantics.
- [x] Upload and render real GoCraft chunk-mesher output through a WebGPU frame pass.
- [x] Camera/view-projection and depth.
- [x] Real generated multi-chunk terrain with neighbor-aware meshing.
- [x] Atlas texture, sampler, existing UVs, vertex tint/AO/light and cutout alpha discard.
- [x] Validate transparent water/glass pass and distance fog on hardware.
- [x] Move the validated WebGPU world pipeline out of preview tests into an actual frame renderer.
- [x] Reuse live visible-section/frustum selection and normal chunk mesh upload/unload integration.
- [ ] Validate the opt-in `go run . -webgpu` live game path on hardware.
- [ ] Remaining world rendering: animated liquids, entities, mining crack and special non-atlas materials.
- [ ] UI/HUD/inventory/pause overlay parity and frame-timing cleanup.
- [ ] Benchmark against the Raylib/OpenGL baseline and decide whether to continue migration.

## Validation notes

The reference Raylib/OpenGL path has been locally confirmed to build, pass the current tests, and visually render the existing world after the initial seam refactor.

The WebGPU path has been visually confirmed through textured terrain plus water/glass/fog previews. Preview tests intentionally mesh only a vertical slice around the surface, so when the orbiting camera sees below that artificial `yMin` plane it can expose dark cave/geology cutaways; those cutaways are a preview artifact, not evidence of broken indexing or depth.

The live `-webgpu` Playing path must not call Raylib `BeginDrawing`/`EndDrawing`; Raylib only polls input while WebGPU owns presentation. The menu remains OpenGL until entering a game. On exit, the World is closed first so its WebGPU mesh buffers are released before the WebGPU device is destroyed and the mesh backend is reset to OpenGL.

The GLFW/WGL warning observed when closing early probes occurred during Raylib's OpenGL-context teardown after WebGPU had presented into the same HWND. It was not a renderer failure in those probes, but the live integration should still be watched for teardown/context issues during local validation.
# Raylib removal: camera and persistence boundary

## Input and layout boundary

The window owner now captures one input snapshot per frame. Gameplay, session/packet cursor changes and inventory/container interactions consume neutral keys, points and rectangles. The current adapter still calls Raylib; this does not yet replace the native window. The menu text field consumes the same character queue so polling does not swallow its text input.

Creative/survival/container layouts are backend-neutral. Legacy drawing adapts their rectangles at its boundary; WebGPU uses the neutral layout directly. Physics tests now call the neutral movement core, the obsolete Raylib physics wrappers and unused brute-force `findHit` were deleted, and legacy face GPU storage moved out of shared world types. Regression tests cover input edges, character queue order, cursor transitions, camera mouse input and cleared direct-import boundaries. Native window/menu interaction still needs end-to-end verification after replacement.

Gameplay now owns a renderer-neutral `gameCamera`; spawning, aiming, movement and legacy player saves use it directly. Only the legacy draw entry converts it to Raylib. Performance monitoring receives frame time from the main loop and no longer queries the window library. Player save regression tests cover round trips and every truncated byte length without mutating live state.

This is a partial migration, not removal of the module: window/input ownership, menus, UI layout and legacy rendering/tests still require migration. No interactive window behavior was changed or visually verified in this step.

## Native preview and test dependency cleanup

The five WebGPU terrain previews now create native Win32 windows directly, with no Raylib window or OpenGL context. `GOCRAFT_WEBGPU_PREVIEW_FRAMES=3` runs an enabled preview hidden for three frames and resizes before the second frame. Without this setting the preview remains interactive. The texture preview switch is `GOCRAFT_WEBGPU_TEXTURE_PREVIEW` (not TEXTURED).

Smooth-light and mesh-math tests no longer import Raylib. Rotation expectations use an independent float64 Rodrigues formula. The active WebGPU bone-pose evaluator is shared CPU code in `mob_pose.go`; content tests now exercise this path instead of the legacy renderer. Import guards cover all WebGPU source and test files, including future additions.

Raylib remains in go.mod because legacy rendering/resources, older UI/mob previews and the standalone smoke tool still require it. No claim of complete dependency removal is made. Existing saves/settings are unchanged by this migration.

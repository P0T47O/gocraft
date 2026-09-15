# WebGPU renderer experiment

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

The experiment now uses `github.com/gogpu/wgpu v0.34.5` with the pure-Go native backend. The earlier `go-webgpu/webgpu` + `wgpu-native` path was abandoned after its v29 vertex-attribute ABI gap caused native pipeline panics. `wgpu_native.dll` is no longer required for this branch's normal WebGPU path.

`webgpu_transparency_preview_test.go` is the current validation target. It adds a second pipeline for water/glass with alpha blending, depth testing with depth writes disabled, back-to-front transparent batch sorting, plus distance fog shared by opaque and translucent terrain. This newest step still requires local validation before it is considered complete.

## Initial audit

GoCraft already owns explicit OpenGL chunk mesh upload/draw state (`platform/mesh.go`, `render_mesh.go`) instead of relying only on Raylib high-level drawing. CPU-side meshing can therefore survive the migration, but WebGPU is a real backend rewrite rather than a library-name substitution.

Raylib also appears in shared/non-rendering structures (client state, world ray queries, saves, mob server and common types). Those dependencies should be removed incrementally rather than with a flag-day conversion.

## Strategy

1. Keep `main` untouched; all work stays on `experiment/webgpu` until a real world renders and is benchmarked.
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
- [ ] Validate transparent water/glass pass and distance fog on hardware.
- [ ] Move the validated WebGPU world pipeline out of preview tests into an actual frame renderer.
- [ ] Visible-section loop/frustum culling and live chunk upload/unload integration.
- [ ] Remaining world rendering: animated liquids, entities, mining crack and special materials.
- [ ] UI/input/platform parity as needed.
- [ ] Benchmark against the Raylib/OpenGL baseline and decide whether to continue migration.

## Validation notes

The reference Raylib/OpenGL path has been locally confirmed to build, pass the current tests, and visually render the existing world after the initial seam refactor.

The WebGPU path has also been visually confirmed through the textured generated-terrain milestone. Preview tests intentionally mesh only a vertical slice around the surface, so when the orbiting camera sees below that artificial `yMin` plane it can expose dark cave/geology cutaways; those cutaways are a preview artifact, not evidence of broken indexing or depth.

The GLFW/WGL warning observed when closing early probes occurred during Raylib's OpenGL-context teardown after WebGPU had presented into the same HWND. It is not currently treated as a renderer failure. The production WebGPU path will not use Raylib's OpenGL draw/swap loop; window/input ownership will be revisited during frame-loop integration.

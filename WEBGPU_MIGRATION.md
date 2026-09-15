# WebGPU renderer experiment

This branch is an isolated experiment for evaluating a WebGPU rendering backend without destabilizing `main`.

## Current status

The Raylib/OpenGL reference renderer still builds and renders the existing GoCraft world after the first backend-neutral mesh refactor.

The Windows surface smoke probe has now been validated on real hardware: WebGPU successfully presents into the existing Raylib-created HWND. The next probe exercises the actual GoCraft 36-byte `platform.Vertex` layout, WebGPU vertex/index buffer upload and indexed drawing. Run it with `go run ./cmd/webgpu-smoke`; success is a blue window containing a four-corner colored quad.

The legacy `platform.UploadMesh` entry used by the real chunk mesher now routes through the selected `MeshBackend`. The concrete OpenGL upload is private to `OpenGLMeshBackend`, so switching to WebGPU can no longer be bypassed accidentally by the existing chunk upload call site. Rendering state is still OpenGL-specific and is the next larger integration boundary.

The experiment uses `github.com/go-webgpu/webgpu v0.5.5`, which requires Go 1.25 and the `wgpu-native` v29 runtime binary (`wgpu_native.dll` on Windows). The native library can be placed beside the executable / in PATH, or selected with `WGPU_NATIVE_PATH`.

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
- [x] Select and pin the Go WebGPU binding/backend (`go-webgpu/webgpu v0.5.5`, wgpu-native v29).
- [x] Prove WebGPU instance/device/surface presentation on the current Raylib-created HWND.
- [x] Replace staged CPU payloads with real WebGPU vertex/index GPU buffers.
- [x] Prove a WGSL pipeline matching the existing 36-byte vertex semantics in the smoke probe.
- [x] Route the real chunk mesher's existing `UploadMesh` call through `MeshBackend` without changing CPU meshing.
- [ ] Upload and render one real GoCraft chunk through a WebGPU frame pass.
- [ ] Camera/view-projection, depth, atlas texture and vertex tint.
- [ ] Visible chunk loop and culling.
- [ ] Transparent pass and remaining world rendering.
- [ ] UI/input/platform parity as needed.
- [ ] Benchmark against the Raylib/OpenGL baseline and decide whether to continue migration.

## Validation note

The reference Raylib/OpenGL path has been locally confirmed to build, pass the current tests, and visually render the existing world after the initial seam refactor. The plain WebGPU surface clear has also been visually validated on Windows hardware. The indexed mesh probe added after that still requires local validation before claiming the vertex layout/buffer path is correct.

The GLFW/WGL warning observed when closing the surface-only smoke probe occurs during Raylib's OpenGL-context teardown after WebGPU has presented into the same HWND. It is not currently treated as a renderer failure; the production WebGPU path will not use Raylib's OpenGL draw/swap loop, and window/input ownership will be revisited after world rendering works.

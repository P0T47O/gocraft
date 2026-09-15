# WebGPU renderer experiment

This branch is an isolated experiment for evaluating a WebGPU rendering backend without destabilizing `main`.

## Current status

The Raylib/OpenGL reference renderer still builds and renders the existing GoCraft world after the first backend-neutral mesh refactor.

The next milestone is now concrete: `cmd/webgpu-smoke` attaches go-webgpu to the **existing Raylib-created Windows HWND** and presents a WebGPU clear pass without using Raylib's drawing/swap path. This intentionally tests whether Raylib can remain as a temporary window/input layer while rendering moves to WebGPU.

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
- [ ] Prove WebGPU instance/device/surface presentation on the current Raylib-created HWND (`go run ./cmd/webgpu-smoke`).
- [ ] Replace staged CPU payloads with real WebGPU vertex/index GPU buffers.
- [ ] WGSL chunk pipeline matching the existing 36-byte vertex semantics.
- [ ] Upload and render one real GoCraft chunk.
- [ ] Camera/view-projection, depth, atlas texture and vertex tint.
- [ ] Visible chunk loop and culling.
- [ ] Transparent pass and remaining world rendering.
- [ ] UI/input/platform parity as needed.
- [ ] Benchmark against the Raylib/OpenGL baseline and decide whether to continue migration.

## Validation note

The reference Raylib/OpenGL path has now been locally confirmed to build, pass the current tests, and visually render the existing world after the initial seam refactor. WebGPU visual correctness is not yet claimed until the smoke probe presents successfully on real hardware.

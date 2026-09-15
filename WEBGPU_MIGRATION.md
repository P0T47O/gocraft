# WebGPU renderer experiment

This branch is an isolated experiment for evaluating a WebGPU rendering backend without destabilizing `main`.

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

- [ ] Backend-neutral mesh payload/handle boundary; existing renderer remains functional.
- [ ] Select and pin the Go WebGPU binding/backend.
- [ ] Device/surface/window bring-up.
- [ ] WGSL chunk pipeline matching the existing 36-byte vertex semantics.
- [ ] Upload and render one real GoCraft chunk.
- [ ] Camera/view-projection, depth, atlas texture and vertex tint.
- [ ] Visible chunk loop and culling.
- [ ] Transparent pass and remaining world rendering.
- [ ] UI/input/platform parity as needed.
- [ ] Benchmark against the Raylib/OpenGL baseline and decide whether to continue migration.

## First technical seam

The first code change should make the existing mesh API backend-neutral before adding WebGPU. In particular, game/render code should no longer know that a mesh is represented by VAO/VBO/EBO IDs. This gives the WebGPU implementation somewhere clean to attach its vertex/index buffers without changing the chunk mesher.

## Validation note

This branch is being brought up remotely. No claim of visual correctness or local GPU validation should be made until it is run on a real supported graphics environment.

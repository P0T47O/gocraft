# WebGPU experiment code index

This supplements `CODE_INDEX.md` without replacing the main project's complete navigation.

| Function | File / entry |
| --- | --- |
| Migration plan and milestones | `WEBGPU_MIGRATION.md` |
| Backend-neutral vertex and opaque current OpenGL mesh | `platform/mesh.go` |
| `MeshBackend` / `MeshHandle`, upload/draw routing | `platform/renderer.go` |
| `webgpu` build-tag staging backend | `platform/webgpu_backend.go`: `EnableExperimentalWebGPU` |
| Chunk mesh backend-neutral GPU handle | `render_mesh.go` |

Current state: the OpenGL VAO/VBO/EBO identities no longer escape `platform.Mesh`; chunk meshes hold a backend-neutral handle and mesh draws route through the selected backend. The tagged WebGPU backend currently owns copies of CPU vertex/index payloads but intentionally does not draw yet.

Next implementation step: choose/pin the WebGPU Go binding, initialize instance/device/surface, replace staged CPU payloads with real vertex/index GPU buffers, then introduce the chunk WGSL pipeline. Normal Raylib/OpenGL remains the reference path during bring-up.

Local validation note: because this work was authored through the repository connector rather than a GPU-capable checkout, run the normal validation plus `go run ./tools/codeindex -write` after checkout before judging the branch. No visual correctness claim has been made yet.

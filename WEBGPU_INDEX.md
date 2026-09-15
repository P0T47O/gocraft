# WebGPU experiment code index

This supplements `CODE_INDEX.md` without replacing the main project's complete navigation.

| Function | File / entry |
| --- | --- |
| Migration plan and milestones | `WEBGPU_MIGRATION.md` |
| Backend-neutral vertex and current OpenGL mesh | `platform/mesh.go` |
| `MeshBackend` / `MeshHandle` seam | `platform/renderer.go` |
| `webgpu` build-tag scaffold | `platform/webgpu_backend.go`: `EnableExperimentalWebGPU` |
| Chunk mesh backend-neutral GPU handle | `render_mesh.go` |

The next implementation step is WebGPU device/surface initialization and a real vertex/index buffer-backed `MeshHandle`. The normal renderer remains the reference path during bring-up.

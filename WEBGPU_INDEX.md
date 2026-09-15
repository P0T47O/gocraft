# WebGPU experiment code index

This supplements `CODE_INDEX.md` without replacing the main project's complete navigation.

| Function | File / entry |
| --- | --- |
| Migration plan and milestones | `WEBGPU_MIGRATION.md` |
| Backend-neutral vertex and opaque current OpenGL mesh | `platform/mesh.go` |
| `MeshBackend` / `MeshHandle`, upload/draw routing | `platform/renderer.go` |
| Real WebGPU vertex/index buffer backend | `platform/webgpu_backend.go`: `NewWebGPUMeshBackend`, `UploadChecked`, `DrawPass` |
| Windows WebGPU surface + indexed mesh smoke probe | `cmd/webgpu-smoke/main_windows.go` |
| Chunk mesh backend-neutral GPU handle | `render_mesh.go` |

Current state: the Raylib-created HWND can be presented by WebGPU on real Windows hardware. The WebGPU backend now uploads the exact GoCraft `platform.Vertex` payload and uint32 index buffers to GPU memory, owns their release, and can bind/draw them inside a WebGPU render pass. The smoke command uses the same 36-byte layout in WGSL and should show a colored quad if the path is correct.

Next implementation step after local validation: feed one real chunk mesher output into the WebGPU pass, then add camera/view-projection uniforms, depth and atlas sampling. The normal Raylib/OpenGL renderer remains the reference path during bring-up.

Local validation: run `go build .`, `go test ./...`, then `go run ./cmd/webgpu-smoke` with `WGPU_NATIVE_PATH` configured. Because new Go symbols were added through the repository connector, also run `go run ./tools/codeindex -write` and commit the regenerated `CODE_SYMBOLS.md` before considering the branch clean.

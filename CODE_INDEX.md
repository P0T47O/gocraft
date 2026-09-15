# 功能与代码导航

> WebGPU 实验分支说明：本分支正在将 GPU mesh 生命周期从 OpenGL/Raylib 具体实现中抽离。迁移计划见 [WEBGPU_MIGRATION.md](WEBGPU_MIGRATION.md)；后端边界见 [platform/renderer.go](platform/renderer.go)，实验脚手架见 [platform/webgpu_backend.go](platform/webgpu_backend.go)。其余功能导航暂与 main 一致，迁移期间每次职责变化继续维护本索引。

## WebGPU 实验入口

| 功能 | 文件与入口 |
| --- | --- |
| WebGPU 迁移计划、里程碑 | [WEBGPU_MIGRATION.md](WEBGPU_MIGRATION.md) |
| 后端中立 Vertex、当前 OpenGL Mesh | [platform/mesh.go](platform/mesh.go) |
| MeshBackend / MeshHandle 边界 | [platform/renderer.go](platform/renderer.go) |
| `webgpu` build tag 实验后端脚手架 | [platform/webgpu_backend.go](platform/webgpu_backend.go)：`EnableExperimentalWebGPU` |
| ChunkMesh 对后端中立 GPU handle 的持有 | [render_mesh.go](render_mesh.go) |

完整 main 分支功能导航仍可参考提交基线；待 WebGPU bring-up 继续推进时，本实验分支会恢复并更新完整索引。

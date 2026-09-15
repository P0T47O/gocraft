# 功能与代码导航

按功能先查本页，按类型/函数名查 [自动符号索引](CODE_SYMBOLS.md)。
本页随功能演进维护；当前加载流水线与测量方法见 [加载说明](LOADING.md)。

> `experiment/webgpu` 分支新增渲染后端实验：迁移计划见 [WEBGPU_MIGRATION.md](WEBGPU_MIGRATION.md)，实验代码导航见 [WEBGPU_INDEX.md](WEBGPU_INDEX.md)。当前新增 `platform/renderer.go` 的 `MeshBackend` / `MeshHandle` 边界，并由 `render_mesh.go` 持有后端中立 GPU mesh handle；`platform/webgpu_backend.go` 为 `webgpu` build tag 下的 bring-up 脚手架。

## 运行入口与客户端

完整功能导航与 main 基线一致；WebGPU 实验暂不移动既有职责。程序入口为 `main.go`，游戏会话为 `game_session.go`，更新为 `game_update.go`，绘制为 `game_render.go`。

## 服务端与协议

服务端与协议在本实验中保持不变：`server*.go`、`protocol*.go` 及相关玩法协议文件继续沿用 main 基线职责。

## 世界、地形与渲染

| 功能 | 文件 |
| --- | --- |
| World、区块查询与修改 | `world_core.go`、`types.go` |
| 网格任务与快照 | `world_mesh.go` |
| 方块表面网格构建 | `chunk_mesher.go` |
| 后端中立 Vertex、当前 OpenGL Mesh | `platform/mesh.go` |
| GPU mesh 后端边界 | `platform/renderer.go` |
| WebGPU 实验脚手架 | `platform/webgpu_backend.go` |
| ChunkMesh 与当前绘制状态 | `render_mesh.go` |
| 可见区块、透明排序、雾距 | `world_render.go`、`render_cull.go` |
| 资源、纹理、shader | `render_assets.go`、`render_textures.go`、`render_shaders.go` |

## 玩法、界面与存档

本实验暂不改变玩法、UI 与存档职责；相关文件继续沿用 main 基线。Raylib 仍存在于这些区域，后续仅在 WebGPU bring-up 确有需要时逐步隔离，避免一次性大重构。

## 验证与维护规则

- 常规验证仍为 `go test ./...`、`go vet ./...`、`go build .`。
- WebGPU 分支目前处于远程 bring-up 阶段，未宣称视觉或 GPU 正确性。
- GPU 创建/绘制/释放保持在渲染线程。
- 新增/移动符号后应运行 `go run ./tools/codeindex -write`；当前远程 connector 无执行环境，因此 `CODE_SYMBOLS.md` 尚未自动重建，首次本地验证时需要执行。

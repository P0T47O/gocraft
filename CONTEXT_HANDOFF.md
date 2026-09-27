# GoCraft 开发上下文交接（2026-09-27）

这份文档供在另一台电脑上接续 Codex 对话使用。它记录当前项目方向、`feature/distant-lod` 分支状态、已验证的事实和下一步，不替代 [功能索引](CODE_INDEX.md)、[符号索引](CODE_SYMBOLS.md) 或 [LOD 设计说明](LOD.md)。接手者应先读 `AGENTS.md` 与这些索引，再核对最新 Git 状态；本文是截至上述日期的快照。

## 项目与用户约定

- 仓库：`https://github.com/P0T47O/gocraft.git`。Go 编写的 Minecraft 风格体素游戏；Windows 客户端采用原生 Win32 + WebGPU，Raylib 已从源码及依赖图移除。客户端可用 `go run .` 启动，专服用 `go run . -server`。
- 仍处于开发期，没有需要维护兼容性的已发布存档或协议版本；除非用户另有要求，暂不为旧存档/旧数据引入兼容层。
- 用户偏好实际可验证的改动，尤其希望对渲染、世界生成和 LOD 做实机或离屏图像检查；单元测试通过不能等同于画面正确。完成改动后常要求推送，但不要因这份交接文档而推断每次修改都已授权推送。
- 维护 `CODE_INDEX.md` 的功能到文件映射；新增/移动 Go 符号后用 `go run ./tools/codeindex -write` 更新 `CODE_SYMBOLS.md`，交付前运行 `go run ./tools/codeindex -check`。不要无关地提交 `settings.json`、存档、生成的可执行文件、诊断截图或本地材质。
- 本仓库未包含商业材质。若家里电脑画面变成占位纹理，需按 `README_zh.md` 的说明自行准备本地 `textures/`；该目录被忽略，不会随 Git 同步。

## 分支与当前工作

- 当前工作分支是 `feature/distant-lod`，从主线 `9a0c99e` 起建立独立远景 LOD 原型；**尚未合并回 `main`**。家里电脑应检出此分支，而非只运行 `main`。
- 截至写本文档时，本机 HEAD 为 `b538815`（`Document vertical structure LOD limitations`），比 `origin/feature/distant-lod` 超前一个提交。提交并推送本文档时，应连同该提交一起推送。`settings.json` 有本机未提交修改，必须保留原样且不纳入提交。
- 用户当前的核心问题：远景对人造的竖直结构和悬空结构表现如何？已用临时真实体素区块构造竖直黑曜石圆环和无支撑铁平台，验证**当前实现会错误地把圆环孔洞填实、把悬空平台延伸为落地柱**。这不是测试夹具出错，也不是仅靠将 16 格 LOD 改成 4 格就能解决的问题，而是稀疏列只记录最高方块、丢失同一 X/Z 列内占据高度区间的表示缺陷。

## 当前 LOD 机制（简述）

- 完整区块仍负责近景碰撞、交互、实体、洞穴及正确光照。独立地平线只绘制非交互远景，不创建完整 `Chunk`；设置中完整渲染距离默认 24 区块，地平线可选关/64/96/128，默认 96。不要用完整 `RenderDistance=128` 代替地平线测试：全区块加载成本很高。
- 每个 LOD tile 覆盖 128×128 方块，随距离选 4/8/16 格地形采样；相邻层级共边并在切换前渐变高度。种子地形、表层色、水面和共用树锚点生成简化网格；地平线内侧留有下垫层，缓解近景区块未到时的空洞。
- 两个 CPU worker 生成 tile；请求近优先、有限队列和每帧最多四次 GPU 上传。GPU 创建/销毁留在渲染线程，离开世界清理资源。
- 已收到的客户端区块可为人造表面提供稀疏的 8×8 采样覆盖；高于原始地面的采样在单独的顶面/竖直侧面棱柱层绘制，不再把建筑拉成地形尖刺。修改过的 tile 带版本失效并后台重建，旧网格在替换前保持可见。**未收到过的区块中的建筑无法凭种子推断**；需要服务器远景数据协议才可显示。
- 巨型长方体、金字塔、水平圆环已有 CPU 与 WebGPU 诊断，并修过此前的长斜裙边；这不代表任意 3D 建筑都正确。门窗、拱门、竖直圆环、悬空平台、地下室等仍超出“每列一个最高点”的表达能力。

## 复现与验证

在 Windows、支持 Vulkan 的显卡驱动、Go 1.25+ 环境下，在仓库根目录运行。PowerShell 示例：

```powershell
go test . -run '^TestLOD|^TestHorizonDistanceClamp' -count=1
go test . -run '^TestLODVerticalRingAndFloatingPlatformLimits$' -count=1
$env:GOCRAFT_WEBGPU_STRUCTURE_PREVIEW = '1'
go test . -run '^TestWebGPUDistantVerticalAndFloatingStructures$' -v -count=1
Remove-Item Env:GOCRAFT_WEBGPU_STRUCTURE_PREVIEW
```

第三项使用临时内存区块与原生 WebGPU 离屏绘制，在本机生成 `work/lod-vertical-near.png`、`work/lod-vertical-middle.png`、`work/lod-vertical-far.png`；这些 PNG 不受 Git 跟踪，可在家里重新生成。`lod_vertical_structures_test.go` 确认原始体素中圆环中央为空、平台下方为空，同时确认稀疏列只剩最高点。测试**故意记录当前失败形态**：成功退出不代表视觉缺陷已解决。另有 `GOCRAFT_WEBGPU_STRUCTURE_PREVIEW=1 go test . -run TestWebGPUDistantStructuresAtThreeDistances -v` 生成长方体/金字塔/水平圆环的三距离图。

一般工程验证：`go test ./...`、`go vet ./...`、`go build .`、`go run ./tools/codeindex -check`。GPU 图像测试需要真实 Windows 图形设备；是否在家里实机玩到场景、近景/远景是否重叠，仍应单独记录，不能由离屏测试代替。GPU 预览使用临时数据，不会改动玩家存档。

## 下一步优先级与注意事项

1. 若要修竖直环和悬空平台，先设计支持同一 X/Z 采样位置有**多个占据 Y 区间**的紧凑 LOD 结构表示，再对区间的顶/底/侧面做可见面网格；为孔洞、悬空平台及跨 tile 边界补 CPU 与 WebGPU 回归。仅把采样格缩小，不能恢复已经丢弃的垂直信息。评估内存、传输、生成耗时与画面收益后再决定是否做完整体素远景。
2. 做一次真实游戏里完整区块与 LOD 同时可见的交界实机检查；现有离屏预览没有近景完整网格，不能证明交界处没有重叠、双绘或突变。
3. 后续再考虑水岸/材质过渡、色彩与树形的层级切换、磁盘缓存及远处建筑的服务器同步。不要把这些“待办”误记为已实现。
4. 回家接续时，先从 GitHub 拉取本分支，确认包含本交接文档及 `b538815`；本机 `settings.json`、材质、存档和截图不会自动迁移。若继续在新的 Codex 任务中工作，可让它先阅读此文档、`AGENTS.md`、`LOD.md` 和 `CODE_INDEX.md`。

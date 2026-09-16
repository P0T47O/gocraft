# 功能与代码导航

按功能先查本页，按类型/函数名查 [自动符号索引](CODE_SYMBOLS.md)。
本页随功能演进维护；当前加载流水线与测量方法见 [加载说明](LOADING.md)。

## 运行入口与客户端

| 功能 | 文件与入口 |
| --- | --- |
| 程序入口、全局界面状态、主循环（`-webgpu` 实验路径在 Playing 状态接管 present） | [main.go](main.go)：`main`、`updateMenu` |
| 游戏会话建立与退出 | [game_session.go](game_session.go)：`startGame`、`exitGame` |
| 每帧更新、暂停、输入与地形就绪 | [game_update.go](game_update.go)：`updateGame` |
| 客户端缺失区块请求 | [game_streaming.go](game_streaming.go)：`requestMissingChunks` |
| 客户端收到消息后的状态更新 | [game_packets.go](game_packets.go)：`handlePacket` |
| 远程实体插值 | [game_entities.go](game_entities.go) |
| 游戏场景绘制 | [game_render.go](game_render.go) |
| WebGPU Playing 状态接入/退出 | [webgpu_game_windows.go](webgpu_game_windows.go)、[webgpu_game_stub.go](webgpu_game_stub.go) |
| WebGPU Playing 单 pass 合成（world + entities/effects + HUD/text + inventory/container） | [webgpu_gameplay_windows.go](webgpu_gameplay_windows.go)：`DrawGameplay` |
| 客户端连接与收发 | [client.go](client.go) |
| 配置、渲染距离 | [settings.go](settings.go)、[menu_screens.go](menu_screens.go) |

## 服务端与协议

| 功能 | 文件与入口 |
| --- | --- |
| Server/连接类型、创建与生命周期 | [server.go](server.go)：`NewServer`、`Start`、`Stop` |
| TCP 接入、连接关闭、队列与广播 | [server_network.go](server_network.go) |
| 权威 tick、区块回收 | [server_tick.go](server_tick.go)：`Tick` |
| 权威消息处理与玩家操作校验 | [server_packets.go](server_packets.go)：`HandlePacket` |
| 区块请求排队、快照、发送预算 | [server_chunks.go](server_chunks.go) |
| 独立加载调度、近处优先排序缓存、客户端队列反压 | [server_streaming.go](server_streaming.go)、[client.go](client.go)、[server_streaming_test.go](server_streaming_test.go) |
| 光照分段快照、合并与完整请求优先 | [server_chunk_light.go](server_chunk_light.go)、[protocol_light.go](protocol_light.go)、[protocol_light_test.go](protocol_light_test.go) |
| 实体更新、生成、背包同步 | [server_entities.go](server_entities.go) |
| 聊天命令 | [server_commands.go](server_commands.go) |
| 服务端保存编排 | [server_save.go](server_save.go)：`Save` |
| 协议 ID、Packet 接口、读写分帧与分发 | [protocol.go](protocol.go) |
| VarInt、字符串编码 | [protocol_codec.go](protocol_codec.go) |
| 区块与方块消息 | [protocol_world.go](protocol_world.go) |
| 登录、玩家移动与出生点消息 | [protocol_player.go](protocol_player.go) |
| 实体消息 | [protocol_entities.go](protocol_entities.go) |
| 背包、交互、聊天等操作消息 | [protocol_actions.go](protocol_actions.go) |
| 容器/生物/生命消息 | [container_protocol.go](container_protocol.go)、[mob_protocol.go](mob_protocol.go)、[player_vitals.go](player_vitals.go) |

## 世界、地形与渲染

| 功能 | 文件 |
| --- | --- |
| World、区块查询与修改 | [world_core.go](world_core.go)、[types.go](types.go) |
| 线程与关闭、对象池 | [world_lifecycle.go](world_lifecycle.go)、[world_pool.go](world_pool.go) |
| 生成任务、区块填充、矿脉 | [world_gen.go](world_gen.go) |
| 群系权重、高度场、河道、地表与植被规则 | [generation_biomes.go](generation_biomes.go) |
| 地形列、坡度、洞穴和坐标哈希 | [generation_terrain.go](generation_terrain.go) |
| 生成任务内地表/树木共享采样缓存与逐字节一致性对照 | [generation_cache.go](generation_cache.go)、[generation_cache_test.go](generation_cache_test.go) |
| 树锚点与跨区块树冠 | [generation_trees.go](generation_trees.go) |
| 生成机制及限制 | [GENERATION.md](GENERATION.md) |
| 光照传播 | [world_light.go](world_light.go) |
| 六面逐顶点光照、AO、防拐角漏光、四边形对角线选择 | [mesh_lighting.go](mesh_lighting.go)、[mesh_lighting_test.go](mesh_lighting_test.go)、[mesh_lighting_preview_test.go](mesh_lighting_preview_test.go)；范围与实测：[LIGHTING.md](LIGHTING.md) |
| 客户端区块消息应用、射线查询 | [world_packets.go](world_packets.go)、[world_ray.go](world_ray.go) |
| 客户端光照增量、精确网格失效范围与更新合并 | [world_packet_light.go](world_packet_light.go)、[world_mesh_dirty.go](world_mesh_dirty.go)、[world_mesh_dirty_test.go](world_mesh_dirty_test.go) |
| 网格任务与快照 | [world_mesh.go](world_mesh.go) |
| 方块表面网格构建 | [chunk_mesher.go](chunk_mesher.go) |
| 后端中立网格上传接口、OpenGL/WebGPU GPU buffer | [render_mesh.go](render_mesh.go)、[platform/renderer.go](platform/renderer.go)、[platform/mesh.go](platform/mesh.go)、[platform/webgpu_backend.go](platform/webgpu_backend.go) |
| OpenGL 可见区块、透明排序、雾距 | [world_render.go](world_render.go)、[render_cull.go](render_cull.go) |
| WebGPU 世界渲染（surface、depth、atlas、opaque/cutout、water/glass、fog、可见 section 提交） | [webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)、[webgpu_atlas.go](webgpu_atlas.go) |
| WebGPU 实体/效果（远程玩家、掉落物、JSON bone 生物动画、挖掘裂纹） | [webgpu_entities_windows.go](webgpu_entities_windows.go) |
| WebGPU gameplay HUD / 文本（准星、热键栏、生命、像素字体、聊天、debug、暂停/死亡提示） | [webgpu_hud_windows.go](webgpu_hud_windows.go)、[webgpu_text_windows.go](webgpu_text_windows.go) |
| WebGPU 背包/容器与物品工具图标 | [webgpu_inventory_windows.go](webgpu_inventory_windows.go)、[webgpu_container_windows.go](webgpu_container_windows.go) |
| WebGPU 迁移验证/交互预览 | [webgpu_chunk_preview_test.go](webgpu_chunk_preview_test.go)、[webgpu_region_preview_test.go](webgpu_region_preview_test.go)、[webgpu_surface_preview_test.go](webgpu_surface_preview_test.go)、[webgpu_textured_preview_test.go](webgpu_textured_preview_test.go)、[webgpu_transparency_preview_test.go](webgpu_transparency_preview_test.go)、[WEBGPU_MIGRATION.md](WEBGPU_MIGRATION.md) |
| 地形颜色缓存 | [mesh_tint.go](mesh_tint.go) |
| 资源持有、初始化与释放 | [render_assets.go](render_assets.go) |
| 纹理加载、材质、透明像素处理 | [render_textures.go](render_textures.go) |
| 世界纹理 mipmap/各向异性过滤、图集留白与安全层级限制 | [render_filter.go](render_filter.go)、[render_filter_test.go](render_filter_test.go)、[render_atlas.go](render_atlas.go)；硬件检测：[platform/texture_filter.go](platform/texture_filter.go)；即时设置和保存：[settings.go](settings.go)、[menu_screens.go](menu_screens.go) |
| 动画纹理及模型纹理切换 | [render_animation.go](render_animation.go) |
| 面模型、面网格模板 | [render_faces.go](render_faces.go) |
| 物品图标、图集、着色器 | [render_icons.go](render_icons.go)、[render_atlas.go](render_atlas.go)、[render_shaders.go](render_shaders.go) |
| 方块/掉落物绘制 | [render_block.go](render_block.go)、[render_item.go](render_item.go) |
| 性能采样与日志 | [performance_monitor.go](performance_monitor.go) |
| 加载阶段耗时/积压/废弃网格统计、32 半径服务端冷加载实测 | [performance_loading.go](performance_loading.go)、[streaming_load_test.go](streaming_load_test.go)、[LOADING.md](LOADING.md) |

## 玩法、界面与存档

| 功能 | 文件 |
| --- | --- |
| 输入、交互、挖掘入口 | [input.go](input.go) |
| 玩家运动与通用碰撞 | [player_movement.go](player_movement.go)、[actor_physics.go](actor_physics.go) |
| 生命、伤害、死亡、重生与区块保护 | [player_vitals.go](player_vitals.go)、[vitals_ui.go](vitals_ui.go) |
| 方块/物品定义、硬度和工具 | [block_registry.go](block_registry.go)、[item_registry.go](item_registry.go)、[mining_data.go](mining_data.go)、[tool_system.go](tool_system.go) |
| 背包、操作、合成 | [inventory.go](inventory.go)、[inventory_actions.go](inventory_actions.go)、[recipes.go](recipes.go) |
| 箱子/熔炉逻辑与保存 | [containers.go](containers.go) |
| 生物配置、AI、刷新、绘制 | [mob_content.go](mob_content.go)、[mob_simulation.go](mob_simulation.go)、[mob_server.go](mob_server.go)、[mob_render.go](mob_render.go)、content/ |
| 菜单与通用控件 | [menu_screens.go](menu_screens.go)、[ui_menu.go](ui_menu.go)、[ui_theme.go](ui_theme.go)、[ui.go](ui.go) |
| 背包/容器/生命/HUD | [inventory_ui.go](inventory_ui.go)、[container_ui.go](container_ui.go)、[vitals_ui.go](vitals_ui.go)、[hud_ui.go](hud_ui.go)、[render_ui.go](render_ui.go) |
| 世界保存编排、元数据与载入 | [save.go](save.go)、[save_manager.go](save_manager.go) |
| 区块文件与批量存取 | [save_chunks.go](save_chunks.go) |
| 实体、旧玩家文件、玩家背包状态 | [save_entities.go](save_entities.go)、[save_player.go](save_player.go)、[survival.go](survival.go) |
| RLE、调色板、二进制基础编码 | [save_codec.go](save_codec.go) |
| 临时文件写入与替换 | [save_file.go](save_file.go) |

## 验证与维护规则

- 全量：`go test ./...`；静态检查：`go vet ./...`；编译：`go build .`。
- 地形：`generation*_test.go`；独立无图形环境验证：`go run ./tests/generation`。
- 生命周期/网格/光照：`world*_test.go`、`render_mesh_test.go`、`platform/mesh_test.go`。
- 存档与协议：`save_file_test.go`、`protocol_varint_test.go`，各玩法测试也覆盖消息往返。
- 重生与生命：`player_vitals_test.go`；视距：`settings_distance_test.go`、`world_fog_test.go`。
- 界面预览：`*_preview_test.go`；性能：`engine_bench_test.go`。
- WebGPU 实机路径：Windows 下 `go run . -webgpu`，菜单仍由 Raylib/OpenGL 绘制，进入 Playing 后 WebGPU 独占 present；当前 world、实体/掉落物、JSON 生物模型与动画、挖掘裂纹、准星/热键栏/生命 HUD、ASCII 文本/聊天/debug/暂停死亡提示、创造/生存背包和箱子/熔炉容器已迁移；第一人称手持物、完整暂停菜单控件、特殊/动画非 atlas 材质仍待迁移。
- 纹理过滤：`render_filter_test.go` 检查设置往返与图集留白；设置 `GOCRAFT_FILTER_GPU_TEST=1` 后运行 `go test . -run TestTextureFilterGPU`，实测倍率、mipmap 开关回读及 GL 错误。
- 过滤设置默认 mipmap 开启、AF 8×，即时生效并保存；AF 自动限制到硬件能力。图集使用加宽留白，8× 保留 0–4 级，16× 限至 0–3 级；关闭 mipmap 仅停用采样，不释放层级内存。
- 新增功能或移动职责时，同一批修改更新本页；符号清单运行 `go run ./tools/codeindex -write` 更新。
- 提交前运行 `go run ./tools/codeindex -check`，防止符号清单过期。
- 同职责使用同一文件前缀；不为了行数拆开一个紧密相关的小流程。
- 活跃 World 由所属主循环持有；生成 worker 操作私有区块，网格 worker 读取快照。拆文件不改变线程归属。
- GPU 创建/绘制/释放继续留在渲染线程；WebGPU backend 切换发生在 Playing 渲染线程，World 先释放 mesh handle 再释放 WebGPU device。

## 尚未完成的结构改进

`chunk_mesher.go` 的 `buildAllMeshData`、`server_packets.go` 的 `HandlePacket`、
`input.go` 仍包含较大的单体流程。这轮保留其内部实现，避免整理目录时混入算法修改。
后续分别适合抽取面生成策略、按消息域处理函数、输入与交互状态机。
工程仍使用 package main；WebGPU 实验现已接管 Playing 的 world、实体/effects、主要 HUD/text、inventory/container presentation，但第一人称手持物、特殊材质，以及 window/input 与 Raylib 类型解耦仍未完成。
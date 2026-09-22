# 功能与代码导航

按功能先查本页，按类型/函数名查 [自动符号索引](CODE_SYMBOLS.md)。
本页随功能演进维护；当前加载流水线与测量方法见 [加载说明](LOADING.md)。

## 运行入口与客户端

| 功能 | 文件与入口 |
| --- | --- |
| 程序入口、全局界面状态（默认原生 WebGPU） | [main.go](main.go)：`main` |
| 游戏会话建立与退出 | [game_session.go](game_session.go)：`startGame`、`exitGame` |
| 原生图形错误诊断与退出后系统提示（不依赖 GPU） | [graphics_failure.go](graphics_failure.go)、[graphics_failure_windows.go](graphics_failure_windows.go)、[graphics_failure_stub.go](graphics_failure_stub.go)；日志回归 [graphics_failure_test.go](graphics_failure_test.go) |
| 每帧更新、暂停、输入与地形就绪 | [game_update.go](game_update.go)：`updateGame` |
| 后端中立玩家摄像机、朝向初始化与瞄准射线 | [game_camera.go](game_camera.go)、[game_camera_test.go](game_camera_test.go) |
| 后端中立输入帧、文本队列、鼠标捕获边界 | [window_input.go](window_input.go)、[window_input_test.go](window_input_test.go)；原生采集 [window_win32_windows.go](window_win32_windows.go)，每帧采集一次 |
| 客户端缺失区块请求：近处优先、跨帧游标、定期重试、发送反压 | [game_streaming.go](game_streaming.go)：`requestMissingChunks`；回归 [game_streaming_test.go](game_streaming_test.go) |
| 512在途请求窗口、满窗口重试及收包自适应预算（0.25–6ms） | [game_streaming.go](game_streaming.go)、[game_streaming_boundary_test.go](game_streaming_boundary_test.go)、[client_packet_budget.go](client_packet_budget.go)、[client_packet_budget_test.go](client_packet_budget_test.go)、[game_update.go](game_update.go) |
| 请求队列平移复用、重试/传送/断线/视距缩小与会话重置回归，16/32/64 调度微基准 | [game_streaming_boundary_test.go](game_streaming_boundary_test.go) |
| 客户端收到消息后的状态更新 | [game_packets.go](game_packets.go)：`handlePacket` |
| 远程实体插值 | [game_entities.go](game_entities.go) |
| 游戏场景绘制 | [webgpu_gameplay_windows.go](webgpu_gameplay_windows.go) |
| 原生 Windows 窗口与主循环（不初始化 Raylib 窗口） | [window_win32_windows.go](window_win32_windows.go)、[native_game_windows.go](native_game_windows.go)、[native_game_stub.go](native_game_stub.go)、[native_window_state.go](native_window_state.go) |
| WebGPU 主菜单、设置和菜单覆盖层：保序批次与裁剪 | [menu_webgpu_windows.go](menu_webgpu_windows.go) |
| 分辨率切换：帧边界应用窗口尺寸、交换链过期跳帧并重建（菜单/游戏共用） | [window_win32_windows.go](window_win32_windows.go)：`applyPendingResize`；[webgpu_surface_windows.go](webgpu_surface_windows.go)、[webgpu_surface_windows_test.go](webgpu_surface_windows_test.go)；实机回归 [native_window_windows_test.go](native_window_windows_test.go) |
| 原生窗口/GPU/临时存档双会话回归与图像回读 | [native_window_windows_test.go](native_window_windows_test.go)、[menu_preview_webgpu_windows_test.go](menu_preview_webgpu_windows_test.go)；设置 `GOCRAFT_WEBGPU_REGRESSION=1` |
| WebGPU 原生帧快照与渲染器生命周期 | [webgpu_game_windows.go](webgpu_game_windows.go)、[webgpu_game_stub.go](webgpu_game_stub.go) |
| WebGPU Playing 单 pass 合成（world + entities/effects + HUD/text + inventory/container + 暂停/死亡菜单） | [webgpu_gameplay_windows.go](webgpu_gameplay_windows.go)：`DrawGameplay` |
| 后端中立帧快照与窗口设置桥接 | [webgpu_frame.go](webgpu_frame.go)、[settings_window.go](settings_window.go)；配置读写本身在 Raylib-free 的 [settings.go](settings.go) |
| 客户端连接与收发（网络层不依赖 Raylib；位置/朝向以标量传入） | [client.go](client.go) |
| 配置、渲染距离（8–128 区块半径，默认24；超过32提示高内存占用） | [settings.go](settings.go)、[menu_screens.go](menu_screens.go)；远裁剪随半径扩展：[world_render.go](world_render.go)、[webgpu_camera_windows.go](webgpu_camera_windows.go) |

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
| 保存错误汇总、退出等待完成与失败日志 | [server_save_result_test.go](server_save_result_test.go)、[session_shutdown.go](session_shutdown.go)、[session_shutdown_test.go](session_shutdown_test.go)、[save_failure.go](save_failure.go)；服务端关闭结果经 Done 同步后由 [game_session.go](game_session.go) 读取 |
| 协议 ID、Packet 接口、读写分帧与分发 | [protocol.go](protocol.go) |
| VarInt、字符串编码 | [protocol_codec.go](protocol_codec.go) |
| 区块与方块消息 | [protocol_world.go](protocol_world.go) |
| 登录、玩家移动与出生点消息 | [protocol_player.go](protocol_player.go) |
| 实体消息 | [protocol_entities.go](protocol_entities.go) |
| 背包、交互、聊天等操作消息 | [protocol_actions.go](protocol_actions.go) |
| 容器/生物/生命消息；生物攻击距离/AABB 权威校验与玩家生命 tick 已不依赖 Raylib | [container_protocol.go](container_protocol.go)、[mob_protocol.go](mob_protocol.go)、[player_vitals.go](player_vitals.go) |

## 世界、地形与渲染

| 功能 | 文件 |
| --- | --- |
| World、区块查询与修改 | [world_core.go](world_core.go)、[types.go](types.go) |
| 稀疏16³区块字段、统一值段、展开/压缩和密集存档适配 | [chunk_storage.go](chunk_storage.go)、[save_chunks.go](save_chunks.go)、[world_packets.go](world_packets.go)；正确性/内存/性能测试：[chunk_storage_test.go](chunk_storage_test.go)、[chunk_storage_equivalence_test.go](chunk_storage_equivalence_test.go)；收益与代价：[CHUNK_STORAGE.md](CHUNK_STORAGE.md) |
| 稀疏列批量复制到网格快照、首次收包同步统计高度/火把/分段非空数 | [chunk_storage.go](chunk_storage.go)：`CopyColumn`；[world_mesh.go](world_mesh.go)、[world_packets.go](world_packets.go)；等价回归 [chunk_storage_batch_test.go](chunk_storage_batch_test.go) |
| 线程与关闭、对象池 | [world_lifecycle.go](world_lifecycle.go)、[world_pool.go](world_pool.go) |
| 生成任务、区块填充、矿脉 | [world_gen.go](world_gen.go) |
| 群系权重、高度场、河道、地表与植被规则 | [generation_biomes.go](generation_biomes.go) |
| 地形列、坡度、洞穴和坐标哈希 | [generation_terrain.go](generation_terrain.go) |
| 生成任务内地表/树木共享采样缓存与逐字节一致性对照 | [generation_cache.go](generation_cache.go)、[generation_cache_test.go](generation_cache_test.go) |
| 树锚点与跨区块树冠 | [generation_trees.go](generation_trees.go) |
| 生成机制及限制 | [GENERATION.md](GENERATION.md) |
| 光照传播 | [world_light.go](world_light.go) |
| 六面逐顶点光照、AO、防拐角漏光、四边形对角线选择 | [mesh_lighting.go](mesh_lighting.go)、[mesh_lighting_test.go](mesh_lighting_test.go)、[mesh_lighting_preview_test.go](mesh_lighting_preview_test.go)；范围与实测：[LIGHTING.md](LIGHTING.md) |
| 客户端区块消息应用、后端中立 DDA 射线查询与 gameplay-vector HitTest 外壳 | [world_packets.go](world_packets.go)、[world_ray.go](world_ray.go)、[world_ray_game.go](world_ray_game.go) |
| 客户端光照增量、精确网格失效范围与更新合并 | [world_packet_light.go](world_packet_light.go)、[world_mesh_dirty.go](world_mesh_dirty.go)、[world_mesh_dirty_test.go](world_mesh_dirty_test.go) |
| 网格任务与快照 | [world_mesh.go](world_mesh.go) |
| 方块表面网格构建 | [chunk_mesher.go](chunk_mesher.go) |
| Raylib-free 网格数学与颜色、GPU 上传适配 | [mesh_math.go](mesh_math.go)、[mesh_color.go](mesh_color.go)、[render_mesh_upload.go](render_mesh_upload.go)；共享句柄 [render_mesh.go](render_mesh.go) |
| WebGPU 24 字节顶点打包（诊断预览可保持 36 字节） | [platform/compact_vertex.go](platform/compact_vertex.go)、[platform/compact_vertex_test.go](platform/compact_vertex_test.go) |
| 后端中立网格上传接口、WebGPU GPU buffer | [render_mesh.go](render_mesh.go)、[platform/renderer.go](platform/renderer.go)、[platform/mesh.go](platform/mesh.go)、[platform/webgpu_backend.go](platform/webgpu_backend.go) |
| 共享可见区块、透明排序、雾距 | [world_render.go](world_render.go)、[render_cull.go](render_cull.go) |
| WebGPU 世界渲染（camera matrices、scene uniform、surface/depth、atlas、atlas 内水/岩浆逐帧更新、opaque/cutout、water/glass、fog、可见 section 提交） | [webgpu_camera_windows.go](webgpu_camera_windows.go)、[webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)、[webgpu_atlas.go](webgpu_atlas.go)、[webgpu_animation_windows.go](webgpu_animation_windows.go) |
| WebGPU 实体/效果（远程玩家、掉落物、JSON bone 生物动画、挖掘裂纹） | [webgpu_entities_windows.go](webgpu_entities_windows.go) |
| WebGPU 实体保守视锥/距离剔除、超容量分批与帧上传缓存 | [webgpu_entity_bounds_windows.go](webgpu_entity_bounds_windows.go)、[webgpu_entities_windows.go](webgpu_entities_windows.go)、[webgpu_upload_windows.go](webgpu_upload_windows.go)；同帧不同批次不能重写同一个目标缓冲区 |
| WebGPU mipmap/AF 设置联动、动画完整留白和 mip 更新、独立 UI 最近邻采样 | [texture_policy.go](texture_policy.go)、[webgpu_mipmap.go](webgpu_mipmap.go)、[webgpu_filter_windows.go](webgpu_filter_windows.go)、[webgpu_animation_windows.go](webgpu_animation_windows.go) |
| WebGPU 优化回归：CPU 数学等价、mip/动画留白、GPU 像素回读、1500 实体分批 | [webgpu_optimization_test.go](webgpu_optimization_test.go)、[webgpu_regression_windows_test.go](webgpu_regression_windows_test.go)；实机设置 `GOCRAFT_WEBGPU_REGRESSION=1` |
| WebGPU 近景像素采样、mipmap 关闭时的单层视图、多帧移动/动画与 LOD 回读 | [webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)：`sample_atlas`；[webgpu_filter_windows.go](webgpu_filter_windows.go)：`filteredAtlasView`；[webgpu_mip_regression_windows_test.go](webgpu_mip_regression_windows_test.go) |
| WebGPU gameplay HUD / 文本（准星、热键栏、生命、像素字体、聊天、debug、暂停/死亡提示） | [webgpu_hud_windows.go](webgpu_hud_windows.go)、[webgpu_text_windows.go](webgpu_text_windows.go) |
| WebGPU 背包/容器与物品工具图标 | [webgpu_inventory_windows.go](webgpu_inventory_windows.go)、[webgpu_container_windows.go](webgpu_container_windows.go) |
| WebGPU 原生窗口迁移验证/交互预览（不再依赖 Raylib 窗口） | [webgpu_chunk_preview_test.go](webgpu_chunk_preview_test.go)、[webgpu_region_preview_test.go](webgpu_region_preview_test.go)、[webgpu_surface_preview_test.go](webgpu_surface_preview_test.go)、[webgpu_textured_preview_test.go](webgpu_textured_preview_test.go)、[webgpu_transparency_preview_test.go](webgpu_transparency_preview_test.go)、[WEBGPU_MIGRATION.md](WEBGPU_MIGRATION.md) |
| 地形颜色缓存 | [mesh_tint.go](mesh_tint.go) |
| 只读 CPU 图集元数据（GPU 资源由 WebGPU renderer 持有） | [render_assets.go](render_assets.go) |
| CPU 纹理加载、图集及透明像素处理 | [webgpu_atlas.go](webgpu_atlas.go) |
| 世界纹理 mipmap/各向异性过滤、图集留白与安全层级限制 | [render_filter.go](render_filter.go)、[render_filter_test.go](render_filter_test.go)、[webgpu_atlas.go](webgpu_atlas.go)；采样器：[webgpu_filter_windows.go](webgpu_filter_windows.go)；即时设置和保存：[settings.go](settings.go)、[menu_screens.go](menu_screens.go) |
| 动画纹理及模型纹理切换 | [webgpu_animation_windows.go](webgpu_animation_windows.go) |
| 面模型、面网格模板 | [chunk_mesher.go](chunk_mesher.go) |
| 物品图标、图集、着色器 | [webgpu_hud_windows.go](webgpu_hud_windows.go)、[webgpu_atlas.go](webgpu_atlas.go)、[webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go) |
| 方块/掉落物绘制 | [webgpu_entities_windows.go](webgpu_entities_windows.go) |
| 性能采样与日志（不依赖窗口库；主循环通过 `UpdateFrame` 传入帧耗时） | [performance_monitor.go](performance_monitor.go) |
| 加载阶段耗时/积压/废弃网格统计、32 半径服务端冷加载实测 | [performance_loading.go](performance_loading.go)、[streaming_load_test.go](streaming_load_test.go)、[LOADING.md](LOADING.md) |
| 128半径真实TCP/网格/WebGPU受限压力测试及瓶颈报告 | [streaming_stress_windows_test.go](streaming_stress_windows_test.go)、[STRESS128.md](STRESS128.md)；`GOCRAFT_STRESS128=1`，临时世界、45秒/内存保护，不是满范围验收 |
| 请求到收包/网格就绪延迟、世界网格缓冲区占用与上传字节数 | [performance_streaming.go](performance_streaming.go)、[performance_loading.go](performance_loading.go)、[platform/mesh.go](platform/mesh.go)、[platform/webgpu_backend.go](platform/webgpu_backend.go)；计数释放回归 [platform/mesh_metrics_test.go](platform/mesh_metrics_test.go) |

## 玩法、界面与存档

| 功能 | 文件 |
| --- | --- |
| 输入、交互、挖掘入口（读取中立输入帧，无 Raylib import）与玩家眼高碰撞 | [input.go](input.go)、[player_collision.go](player_collision.go)；旧 Raylib 物理适配已删除，生命/生物测试直接使用中立向量 |
| 后端中立 gameplay 向量、玩家运动与通用碰撞 | [game_math.go](game_math.go)、[player_movement.go](player_movement.go)、[actor_physics.go](actor_physics.go) |
| 生命、伤害、死亡、重生与区块保护（服务端生命 tick 已使用后端中立向量） | [player_vitals.go](player_vitals.go)、[player_respawn.go](player_respawn.go) |
| 方块/物品定义、硬度和工具 | [block_registry.go](block_registry.go)、[item_registry.go](item_registry.go)、[mining_data.go](mining_data.go)、[tool_system.go](tool_system.go) |
| 背包、操作、合成 | [inventory.go](inventory.go)、[inventory_actions.go](inventory_actions.go)、[recipes.go](recipes.go) |
| 箱子/熔炉逻辑与保存 | [containers.go](containers.go) |
| 生物配置、AI/移动/刷新、CPU 姿态与原生工作台调试包围盒 | [mob_content.go](mob_content.go)、[mob_simulation.go](mob_simulation.go)、[mob_server.go](mob_server.go)、[mob_pose.go](mob_pose.go)、[webgpu_workshop_windows.go](webgpu_workshop_windows.go)、content/ |
| 后端中立菜单与通用控件（窗口/输入/绘制由接口提供） | [menu_screens.go](menu_screens.go)、[ui_menu.go](ui_menu.go)、[menu_death.go](menu_death.go)、[menu_painter.go](menu_painter.go)、[ui_palette.go](ui_palette.go) |
| 背包/容器/生命/HUD | [webgpu_inventory_windows.go](webgpu_inventory_windows.go)、[webgpu_container_windows.go](webgpu_container_windows.go)、[webgpu_hud_windows.go](webgpu_hud_windows.go)、[webgpu_text_windows.go](webgpu_text_windows.go) |
| 中立背包/容器交互与布局 | [inventory_input.go](inventory_input.go)、[container_input.go](container_input.go)、[inventory_layout.go](inventory_layout.go)、[ui.go](ui.go)；绘制与点击共用布局 |
| 世界保存编排、元数据与载入（摄像机参数已中立化，无 Raylib import） | [save.go](save.go)、[save_manager.go](save_manager.go) |
| 区块文件与批量存取 | [save_chunks.go](save_chunks.go) |
| 实体、旧玩家文件、玩家背包状态 | [save_entities.go](save_entities.go)、[save_player.go](save_player.go)、[survival.go](survival.go)；中立摄像机存档往返及全部截断长度回归：[game_camera_test.go](game_camera_test.go) |
| RLE、调色板、二进制基础编码 | [save_codec.go](save_codec.go) |
| 稀疏分段直接编解码兼容调色板/RLE，避免存取时密集展开 | [save_sparse_codec.go](save_sparse_codec.go)、[save_sparse_decode.go](save_sparse_decode.go)；逐字节兼容、往返、损坏输入原子性及编解码性能对照 [save_sparse_codec_test.go](save_sparse_codec_test.go) |
| 临时文件写入与替换 | [save_file.go](save_file.go) |

## 验证与维护规则

- 全量：`go test ./...`；静态检查：`go vet ./...`；编译：`go build .`。
- 地形：`generation*_test.go`；独立无图形环境验证：`go run ./tests/generation`。
- 生命周期/网格/光照：`world*_test.go`、`render_mesh_test.go`、`platform/mesh_test.go`。
- 存档与协议：`save_file_test.go`、`protocol_varint_test.go`，各玩法测试也覆盖消息往返。
- 重生与生命：`player_vitals_test.go`；视距：`settings_distance_test.go`、`world_fog_test.go`。
- 界面预览：`*_preview_test.go`；性能：`engine_bench_test.go`。
- WebGPU 实机路径：Windows 下 `go run . -webgpu` 使用独立 Win32 窗口（含原始鼠标输入、焦点、缩放/尺寸事件），主菜单/世界列表/创建/联机/设置、Playing、暂停/死亡菜单全由 WebGPU 显示。退出世界保留窗口与图集；最终退出先关 World 再释放 GPU，最后销毁窗口。客户端仅支持原生 WebGPU；旧 `-webgpu=false` 不再启动旧后端。
- 纹理过滤：`render_filter_test.go` 检查设置往返与图集留白；设置 `GOCRAFT_FILTER_GPU_TEST=1` 后运行 `go test . -run TestTextureFilterGPU`，验证 WebGPU mipmap 开关像素回读、采样倍率及 UI 上传隔离。
- 过滤设置默认 mipmap 开启、AF 8×，即时生效并保存；AF 使用 WebGPU 标准 1/2/4/8/16 倍等级。图集使用加宽留白，8× 保留 0–4 级，16× 限至 0–3 级；关闭 mipmap 仅停用采样，不释放层级内存。
- 新增功能或移动职责时，同一批修改更新本页；符号清单运行 `go run ./tools/codeindex -write` 更新。
- 提交前运行 `go run ./tools/codeindex -check`，防止符号清单过期。
- 同职责使用同一文件前缀；不为了行数拆开一个紧密相关的小流程。
- 活跃 World 由所属主循环持有；生成 worker 操作私有区块，网格 worker 读取快照。拆文件不改变线程归属。
- GPU 创建/绘制/释放继续留在渲染线程；WebGPU backend 在原生窗口初始化后安装，World 先释放 mesh handle 再释放 WebGPU device。

## 迁移状态与剩余限制

Raylib 已从源码导入、模块文件和依赖图移除。旧游戏循环、OpenGL 网格提交、纹理/模型/图标资源与窗口适配已删除；只有历史许可证说明保留。所有截图预览与生物工作台使用原生 WebGPU。客户端暂限 Windows，其他系统仍可使用专用服务端入口。

原生窗口隐藏集成测试已覆盖全部菜单、尺寸变化、键盘/UTF-16 事件、焦点释放、临时存档两次连接和退出以及暂停设置；主菜单通过 GPU 像素回读检查。尚未人工验证真实鼠标高频输入、不同 DPI 显示器间拖动和 IME 组合输入。内置 WebGPU 字体目前仍仅显示 ASCII，其他字符显示问号。

实体模型实例化、大地形缓冲区子分配/间接绘制仍待性能采样后推进。buildAllMeshData、HandlePacket 和 input.go 仍有较大流程，后续可按面策略、消息域和输入状态机拆分。
网格上传仅持有 GPU 句柄；窗口设置仅调用当前窗口所有者的 Resize 接口。
窗口设置委托回归：[settings_window_test.go](settings_window_test.go)。
## 预览与依赖回归

- 五个地形预览共用 [webgpu_preview_window_windows_test.go](webgpu_preview_window_windows_test.go) 的原生窗口。保留各自开关：`GOCRAFT_WEBGPU_CHUNK_PREVIEW`、`GOCRAFT_WEBGPU_REGION_PREVIEW`、`GOCRAFT_WEBGPU_SURFACE_PREVIEW`、`GOCRAFT_WEBGPU_TEXTURE_PREVIEW`、`GOCRAFT_WEBGPU_TRANSPARENCY_PREVIEW`，设为 `1` 启用；另设 `GOCRAFT_WEBGPU_PREVIEW_FRAMES=3` 可隐藏窗口、第二帧调整尺寸、三帧后退出。不设帧数仍为交互预览。
- 当前 WebGPU 生物骨骼姿态计算提取到 [mob_pose.go](mob_pose.go)，[mob_pose_test.go](mob_pose_test.go) 检查父子旋转、偏移、混合归零及缓冲区复用；[mob_test.go](mob_test.go) 验证当前姿态实现而非旧 Raylib 动画。
- [mesh_lighting_test.go](mesh_lighting_test.go) 使用标准颜色和标量行列式；[webgpu_optimization_test.go](webgpu_optimization_test.go) 使用独立 Rodrigues 公式验证旋转，无 Raylib 测试依赖。
- [window_input_test.go](window_input_test.go) 自动扫描所有 `webgpu*.go` 导入，禁止重新引入 Raylib；全仓库与模块依赖保护见 [renderer_dependency_test.go](renderer_dependency_test.go)。
## WebGPU-only 迁移入口

- 原生生物工作台：[mob_preview_windows.go](mob_preview_windows.go)、[mob_preview.go](mob_preview.go)、[webgpu_workshop_windows.go](webgpu_workshop_windows.go)；保留 1–5 动作、A/D 旋转、B 包围盒和 R 热重载，不打开网络或存档。回归：[mob_workshop_windows_test.go](mob_workshop_windows_test.go)。
- 截图公共资源与 GPU 回读：[native_preview_fixture_windows_test.go](native_preview_fixture_windows_test.go)、[menu_preview_webgpu_windows_test.go](menu_preview_webgpu_windows_test.go)。所有 `*_preview_test.go` 使用当前 WebGPU 实现。
- 重生重试使用原生输入帧时钟：[player_respawn.go](player_respawn.go)、[player_respawn_test.go](player_respawn_test.go)。
- smoke 开发命令：[cmd/webgpu-smoke/main_windows.go](cmd/webgpu-smoke/main_windows.go) 从源码树启动原生真实区块预览，不再维护第二套窗口/着色器代码；需系统 Go。`GOCRAFT_WEBGPU_PREVIEW_FRAMES=3` 可自动退出。
- 防回退：[renderer_dependency_test.go](renderer_dependency_test.go) 扫描全仓库 Go imports、go.mod/go.sum；发布前再执行 `go list -deps -test ./...` 确认依赖闭包无 Raylib。

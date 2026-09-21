# 功能与代码导航

按功能先查本页，按类型/函数名查 [自动符号索引](CODE_SYMBOLS.md)。
本页随功能演进维护；当前加载流水线与测量方法见 [加载说明](LOADING.md)。

## 运行入口与客户端

| 功能 | 文件与入口 |
| --- | --- |
| 程序入口、全局界面状态与旧主循环（`-webgpu` 启动独立原生窗口主循环） | [main.go](main.go)：`main`、`updateMenu` |
| 游戏会话建立与退出 | [game_session.go](game_session.go)：`startGame`、`exitGame` |
| 每帧更新、暂停、输入与地形就绪 | [game_update.go](game_update.go)：`updateGame` |
| 后端中立玩家摄像机、朝向初始化与瞄准射线 | [game_camera.go](game_camera.go)、[game_camera_test.go](game_camera_test.go)；旧绘制入口临时转换：[render_camera_raylib.go](render_camera_raylib.go) |
| 后端中立输入帧、文本队列、鼠标捕获边界 | [window_input.go](window_input.go)、[window_input_test.go](window_input_test.go)；原生采集 [window_win32_windows.go](window_win32_windows.go)，旧窗口适配 [window_input_raylib.go](window_input_raylib.go)，每帧采集一次 |
| 客户端缺失区块请求 | [game_streaming.go](game_streaming.go)：`requestMissingChunks` |
| 客户端收到消息后的状态更新 | [game_packets.go](game_packets.go)：`handlePacket` |
| 远程实体插值 | [game_entities.go](game_entities.go) |
| 游戏场景绘制 | [game_render.go](game_render.go) |
| 原生 Windows 窗口与主循环（不初始化 Raylib 窗口） | [window_win32_windows.go](window_win32_windows.go)、[native_game_windows.go](native_game_windows.go)、[native_game_stub.go](native_game_stub.go)、[native_window_state.go](native_window_state.go) |
| WebGPU 主菜单、设置和菜单覆盖层：保序批次与裁剪 | [menu_webgpu_windows.go](menu_webgpu_windows.go) |
| 分辨率切换：帧边界应用窗口尺寸、交换链过期跳帧并重建（菜单/游戏共用） | [window_win32_windows.go](window_win32_windows.go)：`applyPendingResize`；[webgpu_surface_windows.go](webgpu_surface_windows.go)、[webgpu_surface_windows_test.go](webgpu_surface_windows_test.go)；实机回归 [native_window_windows_test.go](native_window_windows_test.go) |
| 原生窗口/GPU/临时存档双会话回归与图像回读 | [native_window_windows_test.go](native_window_windows_test.go)、[menu_preview_webgpu_windows_test.go](menu_preview_webgpu_windows_test.go)；设置 `GOCRAFT_WEBGPU_REGRESSION=1` |
| WebGPU 原生帧快照与渲染器生命周期 | [webgpu_game_windows.go](webgpu_game_windows.go)、[webgpu_game_stub.go](webgpu_game_stub.go) |
| WebGPU Playing 单 pass 合成（world + entities/effects + HUD/text + inventory/container + 暂停/死亡菜单） | [webgpu_gameplay_windows.go](webgpu_gameplay_windows.go)：`DrawGameplay` |
| 后端中立帧快照与窗口设置桥接 | [webgpu_frame.go](webgpu_frame.go)、[settings_window_raylib.go](settings_window_raylib.go)；配置读写本身在 Raylib-free 的 [settings.go](settings.go) |
| 客户端连接与收发（网络层不依赖 Raylib；位置/朝向以标量传入） | [client.go](client.go) |
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
| 容器/生物/生命消息；生物攻击距离/AABB 权威校验与玩家生命 tick 已不依赖 Raylib | [container_protocol.go](container_protocol.go)、[mob_protocol.go](mob_protocol.go)、[player_vitals.go](player_vitals.go) |

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
| 客户端区块消息应用、后端中立 DDA 射线查询与 gameplay-vector HitTest 外壳 | [world_packets.go](world_packets.go)、[world_ray.go](world_ray.go)、[world_ray_raylib.go](world_ray_raylib.go) |
| 客户端光照增量、精确网格失效范围与更新合并 | [world_packet_light.go](world_packet_light.go)、[world_mesh_dirty.go](world_mesh_dirty.go)、[world_mesh_dirty_test.go](world_mesh_dirty_test.go) |
| 网格任务与快照 | [world_mesh.go](world_mesh.go) |
| 方块表面网格构建 | [chunk_mesher.go](chunk_mesher.go) |
| Raylib-free 网格数学与颜色、GPU 上传适配 | [mesh_math.go](mesh_math.go)、[mesh_color.go](mesh_color.go)、[render_mesh_upload.go](render_mesh_upload.go)；共享句柄 [render_mesh.go](render_mesh.go)，OpenGL 绘制 [render_mesh_opengl.go](render_mesh_opengl.go) |
| WebGPU 24 字节顶点打包（旧后端/预览保持 36 字节） | [platform/compact_vertex.go](platform/compact_vertex.go)、[platform/compact_vertex_test.go](platform/compact_vertex_test.go) |
| 后端中立网格上传接口、OpenGL/WebGPU GPU buffer | [render_mesh.go](render_mesh.go)、[platform/renderer.go](platform/renderer.go)、[platform/mesh.go](platform/mesh.go)、[platform/webgpu_backend.go](platform/webgpu_backend.go) |
| OpenGL 可见区块、透明排序、雾距 | [world_render.go](world_render.go)、[render_cull.go](render_cull.go) |
| WebGPU 世界渲染（camera matrices、scene uniform、surface/depth、atlas、atlas 内水/岩浆逐帧更新、opaque/cutout、water/glass、fog、可见 section 提交） | [webgpu_camera_windows.go](webgpu_camera_windows.go)、[webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)、[webgpu_atlas.go](webgpu_atlas.go)、[webgpu_animation_windows.go](webgpu_animation_windows.go) |
| WebGPU 实体/效果（远程玩家、掉落物、JSON bone 生物动画、挖掘裂纹） | [webgpu_entities_windows.go](webgpu_entities_windows.go) |
| WebGPU 实体保守视锥/距离剔除、超容量分批与帧上传缓存 | [webgpu_entity_bounds_windows.go](webgpu_entity_bounds_windows.go)、[webgpu_entities_windows.go](webgpu_entities_windows.go)、[webgpu_upload_windows.go](webgpu_upload_windows.go)；同帧不同批次不能重写同一个目标缓冲区 |
| WebGPU mipmap/AF 设置联动、动画完整留白和 mip 更新、独立 UI 最近邻采样 | [texture_policy.go](texture_policy.go)、[webgpu_mipmap.go](webgpu_mipmap.go)、[webgpu_filter_windows.go](webgpu_filter_windows.go)、[webgpu_animation_windows.go](webgpu_animation_windows.go) |
| WebGPU 优化回归：CPU 数学等价、mip/动画留白、GPU 像素回读、1500 实体分批 | [webgpu_optimization_test.go](webgpu_optimization_test.go)、[webgpu_regression_windows_test.go](webgpu_regression_windows_test.go)；实机设置 `GOCRAFT_WEBGPU_REGRESSION=1` |
| WebGPU 近景像素采样、mipmap 关闭时的单层视图、多帧移动/动画与 LOD 回读 | [webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)：`sample_atlas`；[webgpu_filter_windows.go](webgpu_filter_windows.go)：`filteredAtlasView`；[webgpu_mip_regression_windows_test.go](webgpu_mip_regression_windows_test.go) |
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
| 性能采样与日志（不依赖窗口库；主循环通过 `UpdateFrame` 传入帧耗时） | [performance_monitor.go](performance_monitor.go) |
| 加载阶段耗时/积压/废弃网格统计、32 半径服务端冷加载实测 | [performance_loading.go](performance_loading.go)、[streaming_load_test.go](streaming_load_test.go)、[LOADING.md](LOADING.md) |

## 玩法、界面与存档

| 功能 | 文件 |
| --- | --- |
| 输入、交互、挖掘入口（读取中立输入帧，无 Raylib import）与玩家眼高碰撞 | [input.go](input.go)、[player_collision.go](player_collision.go)；旧 Raylib 物理适配已删除，生命/生物测试直接使用中立向量 |
| 后端中立 gameplay 向量、玩家运动与通用碰撞 | [game_math.go](game_math.go)、[player_movement.go](player_movement.go)、[actor_physics.go](actor_physics.go) |
| 生命、伤害、死亡、重生与区块保护（服务端生命 tick 已使用后端中立向量） | [player_vitals.go](player_vitals.go)、[vitals_ui.go](vitals_ui.go) |
| 方块/物品定义、硬度和工具 | [block_registry.go](block_registry.go)、[item_registry.go](item_registry.go)、[mining_data.go](mining_data.go)、[tool_system.go](tool_system.go) |
| 背包、操作、合成 | [inventory.go](inventory.go)、[inventory_actions.go](inventory_actions.go)、[recipes.go](recipes.go) |
| 箱子/熔炉逻辑与保存 | [containers.go](containers.go) |
| 生物配置、Raylib-free AI/移动/刷新与 reference renderer 调试包围盒 | [mob_content.go](mob_content.go)、[mob_simulation.go](mob_simulation.go)、[mob_server.go](mob_server.go)、[mob_render.go](mob_render.go)、[mob_render_bounds.go](mob_render_bounds.go)、content/ |
| 后端中立菜单与通用控件（窗口/输入/绘制由接口提供） | [menu_screens.go](menu_screens.go)、[ui_menu.go](ui_menu.go)、[menu_death.go](menu_death.go)、[menu_painter.go](menu_painter.go)、[ui_palette.go](ui_palette.go)；旧绘制 [menu_painter_raylib.go](menu_painter_raylib.go) |
| 背包/容器/生命/HUD | [inventory_ui.go](inventory_ui.go)、[container_ui.go](container_ui.go)、[vitals_ui.go](vitals_ui.go)、[hud_ui.go](hud_ui.go)、[render_ui.go](render_ui.go) |
| 中立背包/容器交互与布局 | [inventory_input.go](inventory_input.go)、[container_input.go](container_input.go)、[inventory_layout.go](inventory_layout.go)、[ui.go](ui.go)；绘制与点击共用布局，旧绘制转换仅在 [inventory_layout_raylib.go](inventory_layout_raylib.go)、[ui_layout_raylib.go](ui_layout_raylib.go) |
| 世界保存编排、元数据与载入（摄像机参数已中立化，无 Raylib import） | [save.go](save.go)、[save_manager.go](save_manager.go) |
| 区块文件与批量存取 | [save_chunks.go](save_chunks.go) |
| 实体、旧玩家文件、玩家背包状态 | [save_entities.go](save_entities.go)、[save_player.go](save_player.go)、[survival.go](survival.go)；中立摄像机存档往返及全部截断长度回归：[game_camera_test.go](game_camera_test.go) |
| RLE、调色板、二进制基础编码 | [save_codec.go](save_codec.go) |
| 临时文件写入与替换 | [save_file.go](save_file.go) |

## 验证与维护规则

- 全量：`go test ./...`；静态检查：`go vet ./...`；编译：`go build .`。
- 地形：`generation*_test.go`；独立无图形环境验证：`go run ./tests/generation`。
- 生命周期/网格/光照：`world*_test.go`、`render_mesh_test.go`、`platform/mesh_test.go`。
- 存档与协议：`save_file_test.go`、`protocol_varint_test.go`，各玩法测试也覆盖消息往返。
- 重生与生命：`player_vitals_test.go`；视距：`settings_distance_test.go`、`world_fog_test.go`。
- 界面预览：`*_preview_test.go`；性能：`engine_bench_test.go`。
- WebGPU 实机路径：Windows 下 `go run . -webgpu` 使用独立 Win32 窗口（含原始鼠标输入、焦点、缩放/尺寸事件），主菜单/世界列表/创建/联机/设置、Playing、暂停/死亡菜单全由 WebGPU 显示。退出世界保留窗口与图集；最终退出先关 World 再释放 GPU，最后销毁窗口。未带参数仍是旧 Raylib 路径，模块依赖尚未删除。
- 纹理过滤：`render_filter_test.go` 检查设置往返与图集留白；设置 `GOCRAFT_FILTER_GPU_TEST=1` 后运行 `go test . -run TestTextureFilterGPU`，实测倍率、mipmap 开关回读及 GL 错误。
- 过滤设置默认 mipmap 开启、AF 8×，即时生效并保存；AF 自动限制到硬件能力。图集使用加宽留白，8× 保留 0–4 级，16× 限至 0–3 级；关闭 mipmap 仅停用采样，不释放层级内存。
- 新增功能或移动职责时，同一批修改更新本页；符号清单运行 `go run ./tools/codeindex -write` 更新。
- 提交前运行 `go run ./tools/codeindex -check`，防止符号清单过期。
- 同职责使用同一文件前缀；不为了行数拆开一个紧密相关的小流程。
- 活跃 World 由所属主循环持有；生成 worker 操作私有区块，网格 worker 读取快照。拆文件不改变线程归属。
- GPU 创建/绘制/释放继续留在渲染线程；WebGPU backend 切换发生在 Playing 渲染线程，World 先释放 mesh handle 再释放 WebGPU device。

## 尚未完成的结构改进

原生 WebGPU 运行路径已不调用 Raylib 窗口、输入和菜单绘制；中立摄像机、物理、存档、输入快照与菜单/背包/容器布局已拆开。保留的旧渲染资源类型、OpenGL 渲染器、Raylib 预览测试和工具仍使模块存在于依赖图中；下一阶段迁移或删除这些旧代码并清理 go.mod/go.sum，不能把当前状态称为依赖清零。

原生窗口隐藏集成测试已覆盖全部菜单、尺寸变化、键盘/UTF-16 事件、焦点释放、临时存档两次连接和退出以及暂停设置；主菜单通过 GPU 像素回读检查。尚未人工验证真实鼠标高频输入、不同 DPI 显示器间拖动和 IME 组合输入。内置 WebGPU 字体目前仍仅显示 ASCII，其他字符显示问号。

实体模型实例化、大地形缓冲区子分配/间接绘制仍待性能采样后推进。buildAllMeshData、HandlePacket 和 input.go 仍有较大流程，后续可按面策略、消息域和输入状态机拆分。

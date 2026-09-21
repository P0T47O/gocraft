# 文件与符号索引（自动生成）

先按功能查找：[CODE_INDEX.md](CODE_INDEX.md)。本表覆盖根目录与 platform 的非测试 Go 文件。

更新：`go run ./tools/codeindex -write`；检查：`go run ./tools/codeindex -check`。不要手工编辑本文件。

## [actor_physics.go](actor_physics.go)

类型：`Collider`

函数/方法：`moveColliderCore`、`colliderHitsCore`

## [assets.go](assets.go)

常量或包级数据定义。

## [block_registry.go](block_registry.go)

类型：`RenderType`、`BlockDef`

函数/方法：`init`、`RegisterBlock`、`GetBlock`、`initBlockRegistry`

## [chunk_mesher.go](chunk_mesher.go)

类型：`MeshBuildData`、`meshBuilder`

函数/方法：`*meshBuilder.addFace`、`*meshBuilder.addFaceSmooth`、`*MeshBuildData.Reset`、`*RenderAssets.isTransparent`、`*RenderAssets.getBiomeBaseColor`、`*RenderAssets.getClimateColor`、`*RenderAssets.shouldDrawFace`、`*RenderAssets.applyAO`、`*RenderAssets.buildAllMeshData`、`allocFloat32`、`allocUint8`、`allocUint16`

## [client.go](client.go)

类型：`Client`

函数/方法：`ConnectTCP`、`*Client.Close`、`*Client.Send`、`*Client.Update`

## [container_input.go](container_input.go)

函数/方法：`*InputState.closeContainerUI`、`*InputState.updateContainerInput`

## [container_protocol.go](container_protocol.go)

类型：`PacketContainerState`、`PacketContainerClick`

函数/方法：`*PacketContainerState.ID`、`*PacketContainerState.Encode`、`*PacketContainerState.Decode`、`*PacketContainerClick.ID`、`*PacketContainerClick.Encode`、`*PacketContainerClick.Decode`

## [containers.go](containers.go)

类型：`BlockPos`、`BlockContainer`、`ContainerSession`

函数/方法：`containerSize`、`smeltResult`、`fuelTicks`、`*BlockContainer.tick`、`*Server.containerReach`、`*Server.openContainer`、`*Server.sendContainer`、`*Server.syncContainer`、`*Server.closeContainer`、`acceptsContainerItem`、`*Server.clickContainer`、`*Server.tickContainers`、`*Server.breakContainer`、`*Server.saveContainers`、`*Server.loadContainers`、`*Server.sendContainerInventory`

## [entities.go](entities.go)

类型：`EntityType`、`Entity`、`BaseEntity`、`PigEntity`、`ItemEntity`、`PlayerEntity`

函数/方法：`*BaseEntity.GetUUID`、`*BaseEntity.GetType`、`*BaseEntity.GetPosition`、`*BaseEntity.SetPosition`、`*BaseEntity.GetRotation`、`*BaseEntity.SetRotation`、`*BaseEntity.IsDirty`、`*BaseEntity.ClearDirty`、`*BaseEntity.Tick`、`*ItemEntity.Tick`、`*PlayerEntity.Tick`

## [frame_time.go](frame_time.go)

函数/方法：`setGameFrameTime`、`gameFrameTime`

## [game_camera.go](game_camera.go)

类型：`gameCamera`

函数/方法：`newGameVec3`、`gameVec3Add`、`gameVec3Subtract`、`*InputState.InitFromCamera`、`*InputState.RayFromCenter`

## [game_entities.go](game_entities.go)

函数/方法：`updateEntities`、`updateInterpolation`

## [game_math.go](game_math.go)

类型：`gameVec3`

函数/方法：`gameVec3Scale`、`gameVec3Length`、`gameVec3Normalize`

## [game_packets.go](game_packets.go)

函数/方法：`handlePacket`

## [game_session.go](game_session.go)

函数/方法：`startGame`、`exitGame`

## [game_streaming.go](game_streaming.go)

函数/方法：`requestMissingChunks`

## [game_update.go](game_update.go)

函数/方法：`updateGame`

## [generation_biomes.go](generation_biomes.go)

类型：`environmentSample`

函数/方法：`terrainNoise`、`sampleEnvironment`、`riverSignedDistance`、`surfaceFromEnvironment`、`vegetationAt`

## [generation_cache.go](generation_cache.go)

类型：`terrainSampleCache`

函数/方法：`*terrainSampleCache.init`、`*terrainSampleCache.column`

## [generation_terrain.go](generation_terrain.go)

类型：`terrainColumn`

函数/方法：`sampleTerrainColumn`、`terrainColumnFromSamples`、`terrainColumn.topY`、`terrainColumn.blockAt`、`generationHash`、`caveNoise`、`caveAt`、`vegetationBlock`

## [generation_trees.go](generation_trees.go)

类型：`treeAnchor`

函数/方法：`sampleTreeAnchor`、`treeAnchorFromColumn`、`treeAnchor.emit`、`generationIsLog`、`generationIsLeaf`、`placeGeneratedTrees`、`placeGeneratedTreesSampled`

## [input.go](input.go)

类型：`InputState`

函数/方法：`NewInputState`、`*InputState.ToggleInventory`、`*InputState.UpdateCamera`、`*InputState.UpdateSelection`、`isSolidBlock`、`blockIndexFromCoord`、`HandleInput`、`*InputState.UpdateInventoryPage`、`*InputState.UpdateInventorySelection`、`collidesWithBlock`

## [inventory.go](inventory.go)

类型：`ItemStack`、`Item`、`Inventory`

函数/方法：`*Inventory.Add`、`*Inventory.AddStack`、`*Inventory.Consume`、`*Inventory.HasItems`、`*Inventory.ConsumeItems`

## [inventory_actions.go](inventory_actions.go)

函数/方法：`*Inventory.Click`、`*Inventory.CountItem`、`*Inventory.Craft`、`craftCapacity`、`recipeBook`

## [inventory_input.go](inventory_input.go)

函数/方法：`inventoryFor`、`selectedBookRecipe`、`recipeAvailable`、`*InputState.updateCraftingInput`

## [inventory_layout.go](inventory_layout.go)

类型：`SurvivalLayout`

函数/方法：`survivalLayout`、`SurvivalLayout.Rect`、`SurvivalLayout.Slot`、`SurvivalLayout.Row`、`containerUISlot`

## [item_registry.go](item_registry.go)

类型：`ItemDef`

函数/方法：`GetItem`、`initBlockItems`、`configureTools`、`GetItemVisual`、`MoveStack`、`StackLimit`、`CanStack`、`*ItemStack.Wear`、`validStack`、`*PlayerEntity.claimPendingItems`、`migratePlayerItems`、`initItemRegistry`

## [main.go](main.go)

类型：`ProgramState`、`MenuPage`、`RemoteEntity`

函数/方法：`main`

## [menu_death.go](menu_death.go)

函数/方法：`drawDeathScreen`

## [menu_painter.go](menu_painter.go)

类型：`menuPainter`

函数/方法：`menuWidth`、`menuHeight`、`menuRect`、`menuRectangle`、`menuLines`、`menuFade`、`menuMeasure`、`menuClip`、`menuUnclip`、`menuGradient`、`SurvivalLayout.Text`、`menuBox`、`menuButton`

## [menu_screens.go](menu_screens.go)

函数/方法：`menuLayout`、`menuNavigate`、`drawMenuBackdrop`、`menuPanel`、`menuMessage`、`drawMenu`、`drawWorldMenu`、`drawCreateMenu`、`drawMultiplayerMenu`、`drawSettingsMenu`、`drawPauseMenu`

## [menu_webgpu_windows.go](menu_webgpu_windows.go)

类型：`gpuMenuBatch`、`gpuMenuPainter`

函数/方法：`*gpuMenuPainter.reset`、`*gpuMenuPainter.Size`、`gpuMenuColor`、`*gpuMenuPainter.batch`、`*gpuMenuPainter.Rect`、`*gpuMenuPainter.Text`、`*gpuMenuPainter.Measure`、`*gpuMenuPainter.Clip`、`*gpuMenuPainter.Unclip`、`*gpuMenuPainter.draw`、`*webGPUWorldRenderer.DrawMenu`

## [mesh_color.go](mesh_color.go)

函数/方法：`meshColor`

## [mesh_lighting.go](mesh_lighting.go)

类型：`litFace`、`faceLightLayout`

函数/方法：`sampleFaceLighting`、`*RenderAssets.applyAOSmooth`、`flipLightDiagonal`

## [mesh_math.go](mesh_math.go)

函数/方法：`meshVec3`、`meshCross`、`meshRotation`、`meshTransform`

## [mesh_tint.go](mesh_tint.go)

类型：`meshTintCache`

函数/方法：`*RenderAssets.buildMeshTintCache`、`*RenderAssets.environmentColor`

## [mining_data.go](mining_data.go)

函数/方法：`configureBasicMining`

## [mob_content.go](mob_content.go)

类型：`MobDefinition`、`MobBone`、`MobModel`、`MobAnimation`、`MobContent`

函数/方法：`mustMobContent`、`loadMobContent`、`reloadMobPreview`、`loadRuntimeMobs`

## [mob_pose.go](mob_pose.go)

类型：`mobBonePose`

函数/方法：`evaluateMobPose`、`rotateXVec`、`addVec3`

## [mob_preview.go](mob_preview.go)

函数/方法：`advanceMobPreview`

## [mob_preview_stub.go](mob_preview_stub.go)

函数/方法：`runMobPreview`

## [mob_preview_windows.go](mob_preview_windows.go)

函数/方法：`runMobPreview`

## [mob_protocol.go](mob_protocol.go)

类型：`PacketMobState`、`PacketAttackMob`

函数/方法：`*PacketMobState.ID`、`*PacketMobState.Encode`、`*PacketMobState.Decode`、`*MobEntity.snapshot`、`*PacketAttackMob.ID`、`*PacketAttackMob.Encode`、`*PacketAttackMob.Decode`、`*Server.attackMob`

## [mob_server.go](mob_server.go)

函数/方法：`*Server.updateMobs`、`*Server.spawnNearbyMobs`

## [mob_simulation.go](mob_simulation.go)

类型：`MobEntity`

函数/方法：`newMob`、`*MobEntity.random`、`*MobEntity.hasBehavior`、`colliderLoaded`、`*MobEntity.Tick`、`*MobEntity.hit`

## [native_game_stub.go](native_game_stub.go)

函数/方法：`runNativeWebGPU`

## [native_game_windows.go](native_game_windows.go)

函数/方法：`runNativeWebGPU`

## [native_window_state.go](native_window_state.go)

常量或包级数据定义。

## [performance_loading.go](performance_loading.go)

类型：`loadingPhase`、`loadingPhaseCounter`

函数/方法：`*PerformanceMonitor.recordLoading`、`loadingCSVHeader`、`*PerformanceMonitor.loadingCSVValues`

## [performance_monitor.go](performance_monitor.go)

类型：`PerformanceMonitor`、`PerfMetrics`

函数/方法：`NewPerformanceMonitor`、`*PerformanceMonitor.Close`、`*PerformanceMonitor.IncrementMeshBuild`、`*PerformanceMonitor.IncrementChunkLoad`、`*PerformanceMonitor.IncrementChunkUnload`、`*PerformanceMonitor.RecordTick`、`framePercentiles`、`*PerformanceMonitor.UpdateFrame`、`*PerformanceMonitor.logMetrics`

## [platform/compact_vertex.go](platform/compact_vertex.go)

类型：`CompactVertex`

函数/方法：`PackCompactVertices`

## [platform/mesh.go](platform/mesh.go)

类型：`Vertex`

函数/方法：`UploadMesh`

## [platform/renderer.go](platform/renderer.go)

类型：`MeshHandle`、`MeshBackend`

函数/方法：`SetMeshBackend`、`UploadRenderMesh`

## [platform/webgpu_backend.go](platform/webgpu_backend.go)

类型：`WebGPUMeshBackend`、`webGPUMesh`

函数/方法：`NewWebGPUMeshBackend`、`*WebGPUMeshBackend.Upload`、`*WebGPUMeshBackend.UploadChecked`、`*WebGPUMeshBackend.DrawPass`、`*webGPUMesh.IndexCount`、`*webGPUMesh.Unload`、`EnableExperimentalWebGPU`

## [player_collision.go](player_collision.go)

函数/方法：`resolveCollision`、`collides`

## [player_movement.go](player_movement.go)

类型：`MovementControls`

函数/方法：`abs32`、`offsetAxis`、`blockAtPosition`、`playerCollides`、`resolvePlayerCollision`、`feetSupportedCore`、`touchesLiquidCore`、`*InputState.stepMovementCore`

## [player_respawn.go](player_respawn.go)

函数/方法：`*InputState.isDead`、`*InputState.updateRespawnRequest`

## [player_vitals.go](player_vitals.go)

类型：`PlayerVitals`、`PacketVitals`、`PacketRespawn`

函数/方法：`*PlayerEntity.initVitals`、`*PlayerEntity.dead`、`*PacketVitals.ID`、`*PacketVitals.Encode`、`*PacketVitals.Decode`、`*PacketRespawn.ID`、`*PacketRespawn.Encode`、`*PacketRespawn.Decode`、`*Server.sendVitals`、`*Server.hurtPlayer`、`*Server.tickPlayerVitals`、`*Server.updatePlayerVitals`、`*Server.pendingRespawnChunks`、`*Server.safeRespawn`、`*Server.respawnPlayer`

## [protocol.go](protocol.go)

类型：`Packet`

函数/方法：`WritePacket`、`ReadPacket`

## [protocol_actions.go](protocol_actions.go)

类型：`PacketClickWindow`、`PacketBlockInteract`、`PacketOpenWindow`、`PacketCraft`、`PacketSlotChange`、`PacketChat`、`PacketGameMode`、`PacketInventoryUpdate`、`PacketPlayerAction`

函数/方法：`*PacketClickWindow.ID`、`*PacketClickWindow.Encode`、`*PacketClickWindow.Decode`、`*PacketBlockInteract.ID`、`*PacketBlockInteract.Encode`、`*PacketBlockInteract.Decode`、`*PacketOpenWindow.ID`、`*PacketOpenWindow.Encode`、`*PacketOpenWindow.Decode`、`*PacketCraft.ID`、`*PacketCraft.Encode`、`*PacketCraft.Decode`、`*PacketSlotChange.ID`、`*PacketSlotChange.Encode`、`*PacketSlotChange.Decode`、`*PacketChat.ID`、`*PacketChat.Encode`、`*PacketChat.Decode`、`*PacketGameMode.ID`、`*PacketGameMode.Encode`、`*PacketGameMode.Decode`、`*PacketInventoryUpdate.ID`、`*PacketInventoryUpdate.Encode`、`*PacketInventoryUpdate.Decode`、`*PacketPlayerAction.ID`、`*PacketPlayerAction.Encode`、`*PacketPlayerAction.Decode`

## [protocol_codec.go](protocol_codec.go)

函数/方法：`WriteVarInt`、`ReadVarInt`、`readVarInt`、`WriteString`、`ReadString`、`ReadVarIntFromReader`

## [protocol_entities.go](protocol_entities.go)

类型：`PacketEntitySpawn`、`PacketEntityDespawn`、`PacketEntityMeta`、`PacketEntityMove`

函数/方法：`*PacketEntitySpawn.ID`、`*PacketEntitySpawn.Encode`、`*PacketEntitySpawn.Decode`、`*PacketEntityDespawn.ID`、`*PacketEntityDespawn.Encode`、`*PacketEntityDespawn.Decode`、`*PacketEntityMeta.ID`、`*PacketEntityMeta.Encode`、`*PacketEntityMeta.Decode`、`*PacketEntityMove.ID`、`*PacketEntityMove.Encode`、`*PacketEntityMove.Decode`

## [protocol_light.go](protocol_light.go)

类型：`PacketChunkLight`

函数/方法：`chunkLightSize`、`*PacketChunkLight.ID`、`*PacketChunkLight.Encode`、`*PacketChunkLight.Decode`

## [protocol_player.go](protocol_player.go)

类型：`PacketLogin`、`PacketPlayerMove`、`PacketSpawnPoint`

函数/方法：`*PacketLogin.ID`、`*PacketLogin.Encode`、`*PacketLogin.Decode`、`*PacketPlayerMove.ID`、`*PacketPlayerMove.Encode`、`*PacketPlayerMove.Decode`、`*PacketSpawnPoint.ID`、`*PacketSpawnPoint.Encode`、`*PacketSpawnPoint.Decode`

## [protocol_world.go](protocol_world.go)

类型：`PacketChunkData`、`PacketBlockChange`、`PacketChunkRequest`、`PacketUnloadChunk`

函数/方法：`*PacketChunkData.ID`、`*PacketChunkData.Encode`、`*PacketChunkData.Decode`、`*PacketBlockChange.ID`、`*PacketBlockChange.Encode`、`*PacketBlockChange.Decode`、`*PacketChunkRequest.ID`、`*PacketChunkRequest.Encode`、`*PacketChunkRequest.Decode`、`*PacketUnloadChunk.ID`、`*PacketUnloadChunk.Encode`、`*PacketUnloadChunk.Decode`

## [recipes.go](recipes.go)

类型：`Recipe`

函数/方法：`RegisterRecipe`、`InitRecipes`、`GetCraftableRecipes`、`CanCraft`

## [render_assets.go](render_assets.go)

类型：`RenderAssets`、`AtlasRect`、`TextureAtlas`

函数/方法：`newWebGPUCPUAssets`、`*RenderAssets.unload`、`*RenderAssets.getAtlasUV`

## [render_cull.go](render_cull.go)

类型：`Frustum`

函数/方法：`ExtractFrustum`、`*Frustum.IntersectsAABB`

## [render_filter.go](render_filter.go)

函数/方法：`supportedAnisotropy`、`atlasSourceCoordinate`

## [render_mesh.go](render_mesh.go)

类型：`ChunkMesh`

函数/方法：`*ChunkMesh.unload`

## [render_mesh_upload.go](render_mesh_upload.go)

函数/方法：`*RenderAssets.applyMeshData`

## [render_types.go](render_types.go)

类型：`BlockGetter`、`LightGetter`、`MetaGetter`

## [save.go](save.go)

类型：`LevelData`

函数/方法：`getZstdEncoder`、`getZstdDecoder`、`ensureSaveDir`、`SaveGame`、`SaveLevelData`、`LoadGame`、`LoadWorld`、`LoadGameIfExists`

## [save_chunks.go](save_chunks.go)

函数/方法：`SaveWorldChunks`、`SaveChunk`、`TryLoadChunk`、`saveChunkFile`、`loadChunkFile`、`loadAllChunks`

## [save_codec.go](save_codec.go)

函数/方法：`ReadUint32`、`encodeChunk`、`decodeChunk`、`rleEncode`、`rleDecode`、`appendUint16`、`appendUint32`、`writeUint32`、`writeInt32`、`writeFloat32`、`readFloat32`

## [save_entities.go](save_entities.go)

函数/方法：`SaveEntities`、`LoadEntities`

## [save_file.go](save_file.go)

函数/方法：`writeSaveFile`、`replaceSaveFile`

## [save_manager.go](save_manager.go)

类型：`SaveInfo`

函数/方法：`ScanSaves`、`CreateNewSave`、`DeleteSave`

## [save_player.go](save_player.go)

函数/方法：`SavePlayerState`、`loadPlayerFile`

## [server.go](server.go)

类型：`Server`、`lastPos`、`ClientConnection`、`PacketWrapper`

函数/方法：`NewServer`、`*Server.Stop`、`*Server.Start`

## [server_chunk_light.go](server_chunk_light.go)

函数/方法：`*Server.queueChunkLightFor`、`chunkLightPacket`

## [server_chunks.go](server_chunks.go)

函数/方法：`*Server.SendChunksAround`、`*Server.queueChunkFor`、`chunkPacket`、`*Server.processPendingChunks`

## [server_commands.go](server_commands.go)

函数/方法：`*Server.SendTo`、`*Server.handleCommand`

## [server_entities.go](server_entities.go)

函数/方法：`*Server.SendInventory`、`*Server.UpdateEntities`、`*Server.SpawnEntity`、`*Server.findPlayerEntity`

## [server_network.go](server_network.go)

函数/方法：`*Server.ListenTCP`、`*Server.StartTCP`、`*Server.ServeTCP`、`*ClientConnection.close`、`*ClientConnection.enqueue`、`*Server.handleNewConnection`、`*Server.Broadcast`、`*Server.BroadcastTo`

## [server_packets.go](server_packets.go)

函数/方法：`*Server.HandlePacket`

## [server_save.go](server_save.go)

函数/方法：`*Server.Save`

## [server_streaming.go](server_streaming.go)

类型：`chunkPriority`

函数/方法：`*Server.processChunkStreaming`、`streamingBatchSize`、`*Server.chunkOrderNeedsRefresh`、`chunkStreamQueue`、`chunkSendHasRoom`

## [server_tick.go](server_tick.go)

函数/方法：`*Server.Tick`

## [settings.go](settings.go)

类型：`GameSettings`

函数/方法：`renderDistance`、`clampRenderDistance`、`LoadSettings`、`SaveSettings`

## [settings_window.go](settings_window.go)

函数/方法：`ApplySettings`

## [survival.go](survival.go)

类型：`survivalPlayerSave`

函数/方法：`saveSurvivalPlayers`、`loadSurvivalPlayers`、`*Server.hasCraftingStation`、`harvestTier`

## [texture_policy.go](texture_policy.go)

函数/方法：`normalizeAnisotropy`、`safeAtlasMipLevel`、`isAnimatedTexture`

## [tool_system.go](tool_system.go)

类型：`ToolType`、`ToolMaterial`

函数/方法：`GetMiningSpeedMultiplier`、`CanHarvest`、`MiningSeconds`

## [types.go](types.go)

类型：`hitInfo`、`blockFaces`

函数/方法：`inBounds`、`divFloor`、`modFloor`

## [ui.go](ui.go)

类型：`InventoryLayout`

函数/方法：`uiScaleFor`、`inventoryScaleFor`、`inventoryLayoutFor`

## [ui_menu.go](ui_menu.go)

类型：`UIComponents`

函数/方法：`NewUIComponents`、`*UIComponents.DrawButton`、`*UIComponents.DrawAction`、`*UIComponents.DrawTextField`、`*UIComponents.DrawLabel`、`*UIComponents.DrawSlider`

## [ui_palette.go](ui_palette.go)

常量或包级数据定义。

## [webgpu_animation_windows.go](webgpu_animation_windows.go)

函数/方法：`updateWebGPUAtlasAnimations`、`resetWebGPUAtlasAnimations`

## [webgpu_atlas.go](webgpu_atlas.go)

类型：`webGPUAtlasAnimation`、`webGPUBlockAtlas`

函数/方法：`webGPUAtlasSourceCoordinate`、`webGPUAtlasTile`、`buildWebGPUBlockAtlas`

## [webgpu_camera_windows.go](webgpu_camera_windows.go)

函数/方法：`webGPUCameraMatrices`、`*webGPUWorldRenderer.updateSceneCamera`、`*webGPUWorldRenderer.collectVisibleCamera`

## [webgpu_container_windows.go](webgpu_container_windows.go)

函数/方法：`drawWebGPUContainerOverlay`、`drawWebGPUFurnaceStatus`

## [webgpu_entities_windows.go](webgpu_entities_windows.go)

类型：`webGPUEntityRenderer`、`webGPUEntityBatch`

函数/方法：`ensureWebGPUEntityRenderer`、`closeWebGPUEntityRenderer`、`*webGPUEntityRenderer.Close`、`newWebGPUEntityBatch`、`webGPUEntityUV`、`webGPUEntitySubUV`、`webGPUEntityItemTexture`、`*webGPUEntityBatch.addQuad`、`rotateY`、`rotateZVec`、`*webGPUEntityBatch.addBox`、`*webGPUEntityBatch.addPlayer`、`*webGPUEntityBatch.addItem`、`webGPUMobFaceUVs`、`*webGPUEntityBatch.addMobBone`、`*webGPUEntityBatch.addMob`、`*webGPUEntityBatch.addMobPlaceholder`、`*webGPUEntityBatch.addMiningCrack`、`*webGPUEntityRenderer.Draw`

## [webgpu_entity_bounds_windows.go](webgpu_entity_bounds_windows.go)

函数/方法：`*webGPUEntityRenderer.entityRadius`

## [webgpu_filter_windows.go](webgpu_filter_windows.go)

函数/方法：`webGPUFilterSettings`、`webGPUAtlasSamplerDescriptor`、`*webGPUWorldRenderer.updateFiltering`、`*webGPUWorldRenderer.filteredAtlasView`

## [webgpu_frame.go](webgpu_frame.go)

类型：`webGPUPoint`、`webGPUVec3`、`webGPUCamera`、`webGPUFrameContext`

## [webgpu_game_stub.go](webgpu_game_stub.go)

函数/方法：`ensureExperimentalWebGPURenderer`、`drawExperimentalWebGPUFrame`、`closeExperimentalWebGPURenderer`

## [webgpu_game_windows.go](webgpu_game_windows.go)

函数/方法：`webGPUMousePosition`、`webGPUFrameFromWindow`、`ensureExperimentalWebGPURenderer`、`drawExperimentalWebGPUFrame`、`closeExperimentalWebGPURenderer`

## [webgpu_gameplay_windows.go](webgpu_gameplay_windows.go)

函数/方法：`*webGPUWorldRenderer.DrawGameplay`

## [webgpu_hud_windows.go](webgpu_hud_windows.go)

类型：`webGPUHUDVertex`、`webGPUHUDRenderer`、`webGPUHUDBuilder`

函数/方法：`ensureWebGPUHUDRenderer`、`closeWebGPUHUDRenderer`、`*webGPUHUDRenderer.Close`、`newWebGPUHUDBuilder`、`*webGPUHUDBuilder.point`、`*webGPUHUDBuilder.rect`、`*webGPUHUDBuilder.texturedRect`、`*webGPUHUDBuilder.border`、`webGPUHUDScale`、`*webGPUHUDBuilder.addCrosshair`、`webGPUHUDHotbarBlock`、`*webGPUHUDBuilder.addBlockIcon`、`*webGPUHUDBuilder.addHotbar`、`*webGPUHUDBuilder.addVitals`、`*webGPUHUDRenderer.Draw`

## [webgpu_inventory_windows.go](webgpu_inventory_windows.go)

函数/方法：`*webGPUHUDRenderer.drawPrepared`、`*webGPUTextRenderer.drawPrepared`、`webGPUItemUV`、`*webGPUHUDBuilder.addItemIcon`、`*webGPUHUDBuilder.addItemStack`、`webGPUAddStackText`、`drawWebGPUInventoryOverlay`、`drawWebGPUCreativeInventory`、`drawWebGPUSurvivalInventory`

## [webgpu_mipmap.go](webgpu_mipmap.go)

类型：`webGPUMip`

函数/方法：`buildWebGPUMips`、`refreshWebGPUMips`、`*webGPUAtlasAnimation.mipFrame`

## [webgpu_surface_windows.go](webgpu_surface_windows.go)

函数/方法：`*webGPUWorldRenderer.prepareSurface`、`*webGPUWorldRenderer.recoverOutdatedSurface`

## [webgpu_text_windows.go](webgpu_text_windows.go)

类型：`webGPUTextVertex`、`webGPUTextRenderer`、`webGPUTextBatch`

函数/方法：`ensureWebGPUTextRenderer`、`closeWebGPUTextRenderer`、`*webGPUTextRenderer.Close`、`newWebGPUTextBatch`、`*webGPUTextBatch.point`、`*webGPUTextBatch.glyph`、`webGPUTextRuneCode`、`*webGPUTextBatch.text`、`*webGPUTextBatch.shadowText`、`webGPUTextWidth`、`*webGPUTextBatch.centered`、`webGPUHotbarItem`、`*webGPUTextBatch.addHotbarText`、`*webGPUTextBatch.addVitalsText`、`*webGPUTextBatch.addChat`、`*webGPUTextBatch.addDebug`、`*webGPUTextBatch.addPauseAndDeath`、`*webGPUTextRenderer.Draw`、`buildWebGPUPixelFontAtlas`

## [webgpu_upload_windows.go](webgpu_upload_windows.go)

类型：`webGPUFrameUploads`

函数/方法：`*webGPUFrameUploads.begin`、`*webGPUFrameUploads.write`、`*webGPUFrameUploads.close`

## [webgpu_workshop_windows.go](webgpu_workshop_windows.go)

函数/方法：`*webGPUEntityBatch.addWorkshopGuides`

## [webgpu_world_renderer_windows.go](webgpu_world_renderer_windows.go)

类型：`webGPUWorldRenderer`

函数/方法：`newWebGPUWorldRenderer`、`*webGPUWorldRenderer.createPipelineResources`、`*webGPUWorldRenderer.configureSurface`、`*webGPUWorldRenderer.drawMeshMap`、`*webGPUWorldRenderer.Close`、`*webGPUWorldRenderer.closeResources`

## [window_input.go](window_input.go)

类型：`uiPoint`、`uiRect`、`windowInputFrame`、`cursorController`

函数/方法：`newUIRect`、`uiContainsPoint`、`inputKeyDown`、`inputKeyPressed`、`inputMouseDown`、`inputMousePressed`、`inputMousePosition`、`inputMouseDelta`、`inputMouseWheel`、`inputTime`、`windowWidth`、`windowHeight`、`inputChar`、`captureCursor`、`releaseCursor`、`positionCursor`、`inputMouseReleased`

## [window_win32_windows.go](window_win32_windows.go)

类型：`winPoint`、`winRect`、`winClass`、`winMessage`、`rawMousePacket`、`nativeWindow`

函数/方法：`nativeUserProc`、`createNativeWindow`、`*nativeWindow.Close`、`*nativeWindow.Resize`、`*nativeWindow.applyPendingResize`、`*nativeWindow.SetPosition`、`*nativeWindow.SetCaptured`、`*nativeWindow.updateCapture`、`*nativeWindow.Poll`、`nativeKey`、`nativeWindowProc`

## [world_core.go](world_core.go)

类型：`World`、`chunkItem`、`chunkKey`、`sectionKey`、`Chunk`

函数/方法：`*World.TickEntities`、`NewFlatWorld`、`*World.StartBackend`、`NewClientWorld`、`*World.allocChunk`、`*World.freeChunk`、`*World.RemoveBlock`、`*World.PlaceAdjacent`、`*World.IsDirty`、`*World.ClearDirty`、`*World.BlockAt`、`*World.MetaAt`、`*World.SetMetaAt`、`*World.SetBlockAt`、`*World.HeightAt`、`*World.markChunkAllSectionsDirty`、`*World.requestChunk`、`*World.getChunkIfGenerated`、`*World.ensureChunk`、`ensureChunkSections`、`sectionIndexForY`、`*Chunk.updateHeightMap`、`*Chunk.rebuildHeightMap`、`*Chunk.rebuildTorchCount`、`*World.UnloadChunks`、`findLandSpawn`

## [world_gen.go](world_gen.go)

类型：`chunkGenJob`、`chunkGenResult`

函数/方法：`init`、`*World.queueChunkGen`、`*World.ProcessGenResults`、`*World.genWorker`、`generateChunkData`、`generateChunkDataSampled`、`terrainTopY`、`blockAtProcedural`、`terrainHeight`、`rawTerrainHeight`、`terrainShapeSample`、`getClimate`、`getBiome`、`isOceanBiome`、`fbm2`、`noise2`、`hash2`、`absInt`、`abs`、`fade`、`lerp`、`smoothstep`、`ridge`、`smoothCurve`、`classifyBiome`、`fastFloor`

## [world_lifecycle.go](world_lifecycle.go)

类型：`blockPos`、`blockEdit`

函数/方法：`*World.deferEdit`、`*World.applyPendingEdits`、`*World.Close`

## [world_light.go](world_light.go)

类型：`lightPos`、`lightChunkCache`、`lightUpdate`

函数/方法：`isOpaqueBlock`、`lightEmission`、`emitsLight`、`lightPos.add`、`*World.LightAt`、`*World.LightSkyAt`、`*World.LightBlockAt`、`initializeChunkLighting`、`localLightPos`、`spreadLocalLight`、`newLightUpdate`、`*lightUpdate.chunk`、`*lightUpdate.at`、`*lightUpdate.level`、`*lightChunkCache.directSky`、`*lightUpdate.mark`、`*lightUpdate.set`、`*lightUpdate.flush`、`*lightUpdate.desired`、`*lightUpdate.settle`、`*World.stitchChunkLighting`、`*lightUpdate.stitch`、`*World.rebuildLightingForChunk`、`*World.updateBlockLight`、`*World.updateSkyLight`、`*World.setBlockLightAtInternal`、`*World.setSkyLightAtInternal`、`*World.setBlockLightAt`

## [world_mesh.go](world_mesh.go)

类型：`meshKind`、`meshSnapshot`、`meshJob`、`meshResult`

函数/方法：`*meshSnapshot.index`、`*meshSnapshot.blockAt`、`*meshSnapshot.lightAt`、`*meshSnapshot.metaAt`、`*meshSnapshot.Release`、`releaseMeshResults`、`*World.StartMeshWorkers`、`buildMeshSnapshotFromNeighbors`、`*World.markChunkSectionDirty`、`*World.markNeighborsDirty`、`*World.requestImmediateMesh`、`*World.requestImmediateAllSections`、`unloadMeshPass`、`clearSectionMeshes`、`setMeshPending`、`*World.submitMesh`、`*World.ProcessImmediateMeshes`、`*World.acceptsMesh`、`*World.ProcessMeshResults`

## [world_mesh_dirty.go](world_mesh_dirty.go)

类型：`chunkMeshChanges`

函数/方法：`*Chunk.invalidateMeshSection`、`*chunkMeshChanges.mark`、`*chunkMeshChanges.apply`

## [world_packet_light.go](world_packet_light.go)

函数/方法：`*World.applyChunkLight`

## [world_packets.go](world_packets.go)

函数/方法：`*World.applyChunkPacket`

## [world_pool.go](world_pool.go)

类型：`ChunkPool`

函数/方法：`NewChunkPool`、`*ChunkPool.Get`、`*ChunkPool.Put`、`*Chunk.Reset`

## [world_ray.go](world_ray.go)

函数/方法：`*World.rayCast`、`sign`、`axisDelta`、`axisMax`

## [world_ray_game.go](world_ray_game.go)

函数/方法：`*World.HitTest`

## [world_render.go](world_render.go)

类型：`worldRenderCache`、`visibleSection`、`translucentDraw`

函数/方法：`worldFogRange`、`*worldRenderCache.radiusOffsets`、`compareVisibleSections`、`compareSectionCoordinates`、`compareTranslucentDraws`、`*worldRenderCache.appendVisibleSections`、`*worldRenderCache.collectTranslucent`


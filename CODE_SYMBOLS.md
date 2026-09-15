# 文件与符号索引（自动生成）

先按功能查找：[CODE_INDEX.md](CODE_INDEX.md)。本表覆盖根目录与 platform 的非测试 Go 文件。

更新：`go run ./tools/codeindex -write`；检查：`go run ./tools/codeindex -check`。不要手工编辑本文件。

## [actor_physics.go](actor_physics.go)

类型：`Collider`

函数/方法：`moveCollider`、`colliderHits`

## [assets.go](assets.go)

常量或包级数据定义。

## [block_registry.go](block_registry.go)

类型：`RenderType`、`BlockDef`

函数/方法：`init`、`RegisterBlock`、`GetBlock`、`initBlockRegistry`

## [chunk_mesher.go](chunk_mesher.go)

类型：`MeshBuildData`、`meshBuilder`

函数/方法：`*meshBuilder.addFace`、`*meshBuilder.addFaceSmooth`、`*MeshBuildData.Reset`、`*RenderAssets.applyMeshData`、`*RenderAssets.isTransparent`、`*RenderAssets.getBiomeBaseColor`、`*RenderAssets.getClimateColor`、`*RenderAssets.shouldDrawFace`、`*RenderAssets.applyAO`、`*RenderAssets.buildAllMeshData`、`allocFloat32`、`allocUint8`、`allocUint16`

## [client.go](client.go)

类型：`Client`

函数/方法：`ConnectTCP`、`*Client.Close`、`*Client.Send`、`*Client.Update`

## [container_protocol.go](container_protocol.go)

类型：`PacketContainerState`、`PacketContainerClick`

函数/方法：`*PacketContainerState.ID`、`*PacketContainerState.Encode`、`*PacketContainerState.Decode`、`*PacketContainerClick.ID`、`*PacketContainerClick.Encode`、`*PacketContainerClick.Decode`

## [container_ui.go](container_ui.go)

函数/方法：`containerUISlot`、`*InputState.closeContainerUI`、`*InputState.updateContainerInput`、`*RenderAssets.drawContainer`

## [containers.go](containers.go)

类型：`BlockPos`、`BlockContainer`、`ContainerSession`

函数/方法：`containerSize`、`smeltResult`、`fuelTicks`、`*BlockContainer.tick`、`*Server.containerReach`、`*Server.openContainer`、`*Server.sendContainer`、`*Server.syncContainer`、`*Server.closeContainer`、`acceptsContainerItem`、`*Server.clickContainer`、`*Server.tickContainers`、`*Server.breakContainer`、`*Server.saveContainers`、`*Server.loadContainers`、`*Server.sendContainerInventory`

## [entities.go](entities.go)

类型：`EntityType`、`Entity`、`BaseEntity`、`PigEntity`、`ItemEntity`、`PlayerEntity`

函数/方法：`*BaseEntity.GetUUID`、`*BaseEntity.GetType`、`*BaseEntity.GetPosition`、`*BaseEntity.SetPosition`、`*BaseEntity.GetRotation`、`*BaseEntity.SetRotation`、`*BaseEntity.IsDirty`、`*BaseEntity.ClearDirty`、`*BaseEntity.Tick`、`*ItemEntity.Tick`、`*PlayerEntity.Tick`

## [game_entities.go](game_entities.go)

函数/方法：`updateEntities`、`updateInterpolation`

## [game_packets.go](game_packets.go)

函数/方法：`handlePacket`

## [game_render.go](game_render.go)

函数/方法：`drawGame`、`drawCharacterModel`

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

## [hud_ui.go](hud_ui.go)

函数/方法：`drawChatOverlay`

## [input.go](input.go)

类型：`InputState`

函数/方法：`NewInputState`、`*InputState.InitFromCamera`、`*InputState.ToggleInventory`、`*InputState.UpdateCamera`、`*InputState.UpdateSelection`、`*InputState.RayFromCenter`、`resolveCollision`、`collides`、`isSolidBlock`、`blockIndexFromCoord`、`HandleInput`、`*InputState.UpdateInventoryPage`、`*InputState.UpdateInventorySelection`、`collidesWithBlock`

## [inventory.go](inventory.go)

类型：`ItemStack`、`Item`、`Inventory`

函数/方法：`*Inventory.Add`、`*Inventory.AddStack`、`*Inventory.Consume`、`*Inventory.HasItems`、`*Inventory.ConsumeItems`

## [inventory_actions.go](inventory_actions.go)

函数/方法：`*Inventory.Click`、`*Inventory.CountItem`、`*Inventory.Craft`、`craftCapacity`、`recipeBook`

## [inventory_ui.go](inventory_ui.go)

类型：`SurvivalLayout`

函数/方法：`survivalLayout`、`SurvivalLayout.Rect`、`SurvivalLayout.Slot`、`SurvivalLayout.Row`、`inventoryFor`、`selectedBookRecipe`、`recipeAvailable`、`*InputState.updateCraftingInput`、`*RenderAssets.drawInventoryItem`、`*RenderAssets.drawSurvivalInventory`

## [item_registry.go](item_registry.go)

类型：`ItemDef`

函数/方法：`GetItem`、`initBlockItems`、`configureTools`、`GetItemVisual`、`MoveStack`、`StackLimit`、`CanStack`、`*ItemStack.Wear`、`validStack`、`*PlayerEntity.claimPendingItems`、`migratePlayerItems`、`initItemRegistry`

## [main.go](main.go)

类型：`ProgramState`、`MenuPage`、`RemoteEntity`

函数/方法：`main`、`updateMenu`

## [menu_screens.go](menu_screens.go)

函数/方法：`menuLayout`、`menuNavigate`、`drawMenuBackdrop`、`menuPanel`、`menuMessage`、`drawMenu`、`drawWorldMenu`、`drawCreateMenu`、`drawMultiplayerMenu`、`drawSettingsMenu`、`drawPauseMenu`

## [mesh_lighting.go](mesh_lighting.go)

类型：`litFace`、`faceLightLayout`

函数/方法：`sampleFaceLighting`、`*RenderAssets.applyAOSmooth`、`flipLightDiagonal`

## [mesh_tint.go](mesh_tint.go)

类型：`meshTintCache`

函数/方法：`*RenderAssets.buildMeshTintCache`、`*RenderAssets.environmentColor`

## [mining_data.go](mining_data.go)

函数/方法：`configureBasicMining`

## [mob_content.go](mob_content.go)

类型：`MobDefinition`、`MobBone`、`MobModel`、`MobAnimation`、`MobContent`

函数/方法：`mustMobContent`、`loadMobContent`、`reloadMobPreview`、`loadRuntimeMobs`

## [mob_preview.go](mob_preview.go)

函数/方法：`runMobPreview`

## [mob_protocol.go](mob_protocol.go)

类型：`PacketMobState`、`PacketAttackMob`

函数/方法：`*PacketMobState.ID`、`*PacketMobState.Encode`、`*PacketMobState.Decode`、`*MobEntity.snapshot`、`*PacketAttackMob.ID`、`*PacketAttackMob.Encode`、`*PacketAttackMob.Decode`、`*Server.attackMob`

## [mob_render.go](mob_render.go)

类型：`MobRenderer`

函数/方法：`newMobRenderer`、`*MobRenderer.Close`、`mobPose`、`*MobRenderer.Draw`

## [mob_server.go](mob_server.go)

函数/方法：`viewDirection`、`*Server.updateMobs`、`*Server.spawnNearbyMobs`

## [mob_simulation.go](mob_simulation.go)

类型：`MobEntity`

函数/方法：`newMob`、`*MobEntity.random`、`*MobEntity.hasBehavior`、`colliderLoaded`、`*MobEntity.Tick`、`mobBox`、`*MobEntity.hit`

## [performance_loading.go](performance_loading.go)

类型：`loadingPhase`、`loadingPhaseCounter`

函数/方法：`*PerformanceMonitor.recordLoading`、`loadingCSVHeader`、`*PerformanceMonitor.loadingCSVValues`

## [performance_monitor.go](performance_monitor.go)

类型：`PerformanceMonitor`、`PerfMetrics`

函数/方法：`NewPerformanceMonitor`、`*PerformanceMonitor.Close`、`*PerformanceMonitor.IncrementMeshBuild`、`*PerformanceMonitor.IncrementChunkLoad`、`*PerformanceMonitor.IncrementChunkUnload`、`*PerformanceMonitor.RecordTick`、`framePercentiles`、`*PerformanceMonitor.Update`、`*PerformanceMonitor.logMetrics`

## [platform/gl.go](platform/gl.go)

函数/方法：`InitGL`、`getProc`、`SetTextureMaxLevel`、`GenBuffer`、`BindBuffer`、`BufferData`、`GenVertexArray`、`BindVertexArray`、`EnableVertexAttribArray`、`VertexAttribPointer`、`DrawElements`、`DeleteBuffer`、`DeleteVertexArray`、`UseProgram`、`UniformMatrix4fv`、`GetUniformLocation`、`ActiveTexture`、`Uniform4f`、`Uniform3f`、`Uniform1f`、`Uniform1i`、`BindTexture`、`PolygonOffset`、`Enable`、`Disable`

## [platform/mesh.go](platform/mesh.go)

类型：`Mesh`、`Vertex`

函数/方法：`UploadMesh`、`*Mesh.Draw`、`*Mesh.Unload`、`InitGLOnce`

## [platform/texture_filter.go](platform/texture_filter.go)

函数/方法：`MaxTextureAnisotropy`

## [player_movement.go](player_movement.go)

类型：`MovementControls`

函数/方法：`abs32`、`offsetAxis`、`blockAtPosition`、`feetSupported`、`touchesLiquid`、`*InputState.StepMovement`

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

## [render_animation.go](render_animation.go)

函数/方法：`isAnimatedTexture`、`*RenderAssets.loadAnimatedTexture`、`*RenderAssets.Update`、`*RenderAssets.setTextureForPath`、`*RenderAssets.loadBlockTextures`、`*RenderAssets.getFaceModelAnimated`、`*RenderAssets.isAnimated`、`*RenderAssets.currentTexture`、`*RenderAssets.baseTexture`、`*RenderAssets.applyTextureToModel`

## [render_assets.go](render_assets.go)

类型：`RenderAssets`、`AnimatedTexture`、`TextureAtlas`

函数/方法：`loadRenderAssets`、`*RenderAssets.unload`

## [render_atlas.go](render_atlas.go)

函数/方法：`*RenderAssets.generateAtlas`、`*RenderAssets.getAtlasUV`

## [render_block.go](render_block.go)

函数/方法：`*RenderAssets.drawBlock`

## [render_cull.go](render_cull.go)

类型：`Frustum`

函数/方法：`ExtractFrustum`、`*Frustum.IntersectsAABB`

## [render_faces.go](render_faces.go)

函数/方法：`*RenderAssets.makeFace`、`*RenderAssets.initFaceMeshes`、`*RenderAssets.getFaceModel`、`*RenderAssets.initFaceModels`

## [render_filter.go](render_filter.go)

函数/方法：`normalizeAnisotropy`、`supportedAnisotropy`、`safeAtlasMipLevel`、`configureWorldTexture`、`applyWorldTextureFiltering`、`atlasSourceCoordinate`

## [render_icons.go](render_icons.go)

函数/方法：`*RenderAssets.initIcons`、`*RenderAssets.makeIcon`、`measureIconOffset`

## [render_item.go](render_item.go)

类型：`meshDataHolder`

函数/方法：`*RenderAssets.DrawItem`、`*RenderAssets.drawBlockItem`、`*RenderAssets.initCrossItemModels`、`*RenderAssets.generateCrossItemMesh`、`addCubeVertices`

## [render_mesh.go](render_mesh.go)

类型：`ChunkMesh`、`meshShaderState`、`meshRenderState`

函数/方法：`*meshRenderState.reset`、`*meshRenderState.draw`、`*ChunkMesh.Draw`、`*ChunkMesh.unload`

## [render_shaders.go](render_shaders.go)

函数/方法：`loadCutoutShader`、`*RenderAssets.loadFogShader`

## [render_textures.go](render_textures.go)

函数/方法：`*RenderAssets.loadTexture`、`*RenderAssets.getMaterial`、`pickEdgeColorKey`

## [render_types.go](render_types.go)

类型：`BlockGetter`、`LightGetter`、`MetaGetter`

## [render_ui.go](render_ui.go)

函数/方法：`*RenderAssets.drawCrosshair`、`*RenderAssets.drawHotbar`、`*RenderAssets.drawInventory`、`*RenderAssets.drawSlotGrid`、`*RenderAssets.drawHotbarSlots`、`*RenderAssets.drawSlot`、`*RenderAssets.drawSlotOverlay`、`*RenderAssets.drawIcon`、`*RenderAssets.drawDraggedIcon`

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

函数/方法：`*Server.processChunkStreaming`、`streamingBatchSize`、`*Server.chunkOrderNeedsRefresh`、`chunkSendHasRoom`

## [server_tick.go](server_tick.go)

函数/方法：`*Server.Tick`

## [settings.go](settings.go)

类型：`GameSettings`

函数/方法：`renderDistance`、`clampRenderDistance`、`LoadSettings`、`SaveSettings`、`ApplySettings`

## [survival.go](survival.go)

类型：`survivalPlayerSave`

函数/方法：`saveSurvivalPlayers`、`loadSurvivalPlayers`、`*Server.hasCraftingStation`、`harvestTier`

## [tool_system.go](tool_system.go)

类型：`ToolType`、`ToolMaterial`

函数/方法：`GetMiningSpeedMultiplier`、`CanHarvest`、`MiningSeconds`

## [types.go](types.go)

类型：`hitInfo`、`blockFaces`、`faceMesh`

函数/方法：`inBounds`、`divFloor`、`modFloor`、`findHit`

## [ui.go](ui.go)

类型：`InventoryLayout`

函数/方法：`uiScale`、`inventoryScale`、`inventoryLayout`

## [ui_menu.go](ui_menu.go)

类型：`UIComponents`

函数/方法：`NewUIComponents`、`*UIComponents.DrawButton`、`*UIComponents.DrawAction`、`*UIComponents.DrawTextField`、`*UIComponents.DrawLabel`、`*UIComponents.DrawSlider`

## [ui_theme.go](ui_theme.go)

函数/方法：`inventoryText`、`SurvivalLayout.Text`、`inventoryBox`、`inventoryButton`

## [vitals_ui.go](vitals_ui.go)

函数/方法：`*InputState.isDead`、`*InputState.updateRespawnRequest`、`drawVitalsHUD`、`drawDeathScreen`

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

函数/方法：`*World.HitTest`、`*World.rayCast`、`sign`、`axisDelta`、`axisMax`

## [world_render.go](world_render.go)

类型：`worldRenderCache`、`visibleSection`、`translucentDraw`

函数/方法：`worldFogRange`、`*worldRenderCache.radiusOffsets`、`compareVisibleSections`、`compareSectionCoordinates`、`compareTranslucentDraws`、`*worldRenderCache.appendVisibleSections`、`*worldRenderCache.collectTranslucent`、`*worldRenderCache.drawMeshes`、`*World.Draw`、`*World.DrawBlockCrack`


package main

import (
	"bytes"
	"net"
	"testing"
	"time"
)

func lifecycleChunk(w *World, key chunkKey) *Chunk {
	c := w.ensureChunk(key.X, key.Z)
	c.generated = true
	ensureChunkSections(c)
	return c
}

func TestMeshSnapshotSurvivesChunkReuse(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{0, 0})
	c.blocks.Set(15, 15, 15, blockStone)
	c.rebuildTorchCount()
	n := lifecycleChunk(w, chunkKey{1, 1})
	n.blocks.Set(0, 16, 0, blockGlass)
	a := &RenderAssets{}
	if !w.submitMesh(0, 0, 0, a) {
		t.Fatal("mesh not submitted")
	}
	job := <-w.meshJobs
	defer job.snapshot.Release()
	if job.neighbors != ([3][3]*Chunk{}) {
		t.Fatal("job retained live chunks")
	}
	w.UnloadChunks(100, 100, 1, nil)
	for i := 0; i < 4; i++ {
		replacement := lifecycleChunk(w, chunkKey{100 + i, 100})
		replacement.blocks.Set(15, 15, 15, blockWater)
	}
	if job.snapshot.blockAt(15, 15, 15) != blockStone || job.snapshot.blockAt(16, 16, 16) != blockGlass {
		t.Fatal("snapshot changed after reuse")
	}
	if w.acceptsMesh(meshResult{key: job.key, section: 0, version: job.version, instance: job.instance, request: job.request}) != nil {
		t.Fatal("unloaded result accepted")
	}
}

func TestMeshDedupAndStaleResultOwnership(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := lifecycleChunk(w, chunkKey{})
	c.blocks.Set(1, 1, 1, blockStone)
	c.rebuildTorchCount()
	a := &RenderAssets{}
	if !w.submitMesh(0, 0, 0, a) || w.submitMesh(0, 0, 0, a) {
		t.Fatal("duplicate jobs")
	}
	job := <-w.meshJobs
	defer job.snapshot.Release()
	res := meshResult{key: job.key, section: 0, version: job.version, instance: job.instance, request: job.request}
	w.markChunkSectionDirty(0, 0, 0)
	if w.acceptsMesh(res) != nil || c.pendingOpaque[0] {
		t.Fatal("stale version did not clear its own pending flag")
	}
	if !w.submitMesh(0, 0, 0, a) {
		t.Fatal("could not resubmit")
	}
	if w.acceptsMesh(res) != nil || !c.pendingOpaque[0] {
		t.Fatal("old result cleared new job pending flag")
	}
}

func TestFullMeshQueueDoesNotLatchPending(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	w.meshJobs = make(chan meshJob, 1)
	c := lifecycleChunk(w, chunkKey{})
	c.blocks.Set(0, 0, 0, blockStone)
	c.rebuildTorchCount()
	w.meshJobs <- meshJob{snapshot: &meshSnapshot{}}
	if w.submitMesh(0, 0, 0, &RenderAssets{}) || c.pendingOpaque[0] {
		t.Fatal("full queue latched pending")
	}
}

func TestWorldCloseUnblocksFullWorkerQueues(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	w.meshResults = make(chan meshResult, 1)
	w.meshResults <- meshResult{}
	w.StartMeshWorkers(&RenderAssets{}, 1)
	w.meshJobs <- meshJob{snapshot: &meshSnapshot{sizeX: 18, sizeY: 18, sizeZ: 18, blocks: make([]byte, 5832), light: make([]byte, 5832), meta: make([]byte, 5832)}, yMax: 0}
	closed := make(chan struct{})
	go func() { w.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("mesh shutdown blocked")
	}
	w.Close()

	g := NewFlatWorld()
	g.genResults = make(chan chunkGenResult, 1)
	g.genResults <- chunkGenResult{chunk: &Chunk{}}
	g.requestChunk(0, 0)
	g.StartBackend()
	closed = make(chan struct{})
	go func() { g.Close(); close(closed) }()
	select {
	case <-closed:
	case <-time.After(3 * time.Second):
		t.Fatal("generation shutdown blocked")
	}
}

func TestGenerationRejectsOldInstanceAndAppliesDeferredEdit(t *testing.T) {
	initBlockRegistry()
	w := NewFlatWorld()
	defer w.Close()
	_ = w.BlockAt(32, 70, 32)
	_ = w.MetaAt(32, 70, 32)
	_ = w.HeightAt(32, 32)
	if len(w.chunks) != 0 || len(w.pending) != 0 {
		t.Fatal("read-only world queries requested chunks")
	}
	old := w.requestChunk(0, 0).instance
	w.UnloadChunks(100, 100, 1, nil)
	c := w.requestChunk(0, 0)
	stale := &Chunk{generated: true}
	stale.blocks.Set(0, 1, 0, blockWater)
	w.genResults <- chunkGenResult{key: chunkKey{}, chunk: stale, instance: old}
	w.ProcessGenResults()
	if c.generated {
		t.Fatal("stale generation replaced new instance")
	}
	w.SetBlockAt(0, 1, 0, blockGlass)
	ready := &Chunk{generated: true}
	initializeChunkLighting(ready)
	ready.rebuildTorchCount()
	w.genResults <- chunkGenResult{key: chunkKey{}, chunk: ready, instance: c.instance}
	w.ProcessGenResults()
	if w.BlockAt(0, 1, 0) != blockGlass || len(w.pendingEdits) != 0 {
		t.Fatal("deferred edit lost")
	}
}

func TestChunkRetryIsNoOpAndMetadataRoundTrips(t *testing.T) {
	initBlockRegistry()
	w := NewClientWorld()
	defer w.Close()
	c := &Chunk{}
	c.blocks.Set(3, 20, 4, blockTorch)
	c.meta.Set(3, 20, 4, 3)
	initializeChunkLighting(c)
	p := chunkPacket(chunkKey{}, c)
	var wire bytes.Buffer
	if err := WritePacket(&wire, p); err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadPacket(&wire)
	if err != nil {
		t.Fatal(err)
	}
	if !w.applyChunkPacket(decoded.(*PacketChunkData)) {
		t.Fatal("invalid packet")
	}
	loaded := w.chunks[chunkKey{}]
	versions := append([]uint32(nil), loaded.meshVersion...)
	w.applyChunkPacket(p)
	for i, v := range versions {
		if loaded.meshVersion[i] != v {
			t.Fatal("duplicate invalidated mesh")
		}
	}
	if loaded.meta.Get(3, 20, 4) != 3 || len(loaded.torches) != 1 {
		t.Fatal("metadata/index lost")
	}
	if w.applyChunkPacket(&PacketChunkData{Data: []byte{0}}) {
		t.Fatal("accepted short data")
	}
}

func TestServerLoginSeedChunkAndShutdown(t *testing.T) {
	initBlockRegistry()
	for cycle := 0; cycle < 2; cycle++ {
		s := NewServer(t.TempDir())
		s.HasSavedPos = true
		s.InitialPosX = 8
		s.InitialPosY = 80
		s.InitialPosZ = 8
		seed := s.World.seed
		if err := s.ListenTCP("127.0.0.1:0"); err != nil {
			t.Fatal(err)
		}
		go s.ServeTCP()
		defer s.Stop()
		conn, err := net.Dial("tcp", s.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(8 * time.Second))
		if err := WritePacket(conn, &PacketLogin{Username: "LifecycleTest", ProtocolVersion: protocolVersion}); err != nil {
			t.Fatal(err)
		}
		pkt, err := ReadPacket(conn)
		if err != nil {
			t.Fatal(err)
		}
		login, ok := pkt.(*PacketLogin)
		if !ok || login.Seed != seed {
			t.Fatalf("first reply not authoritative seed: %T", pkt)
		}
		gotChunk := false
		for !gotChunk {
			pkt, err = ReadPacket(conn)
			if err != nil {
				t.Fatal(err)
			}
			if p, ok := pkt.(*PacketChunkData); ok {
				// The top world layer always receives direct sky in this fixture.
				if p.LightData[(0*chunkHeight+255)*chunkWidth]>>4 != 15 {
					t.Fatal("uninitialized light published")
				}
				gotChunk = true
			}
		}
		idle, err := net.Dial("tcp", s.Listener.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		s.Stop()
		select {
		case <-s.Done:
		case <-time.After(8 * time.Second):
			t.Fatal("server failed to close connections/workers")
		}
		idle.Close()
		conn.Close()
		if len(s.World.chunks) != 0 {
			t.Fatal("world retained chunks after shutdown")
		}
	}
}

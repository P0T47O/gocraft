package platform

import (
	"sync/atomic"
	"testing"
)

func TestMeshAccountingUnloadIdempotent(t *testing.T) {
	beforeCount, beforeBytes := atomic.LoadInt64(&ActiveMeshCount), MeshBufferBytes.Load()
	atomic.AddInt64(&ActiveMeshCount, 1)
	MeshBufferBytes.Add(100)
	m := &webGPUMesh{counted: true, vertexBytes: 60, indexBytes: 40}
	m.Unload()
	m.Unload()
	if atomic.LoadInt64(&ActiveMeshCount) != beforeCount || MeshBufferBytes.Load() != beforeBytes {
		t.Fatal("mesh accounting did not return to baseline")
	}
	(&webGPUMesh{}).Unload()
	if atomic.LoadInt64(&ActiveMeshCount) != beforeCount {
		t.Fatal("empty mesh decremented count")
	}
}

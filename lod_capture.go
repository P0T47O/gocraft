package main

// A capture worker sees only its private voxel copy. The world owner publishes
// versioned records, and edits synchronously supersede unfinished captures.
type lodCaptureJob struct {
	key    chunkKey
	seed   uint32
	id     uint64
	chunk  *Chunk
	record *lodObservedChunk
}

type lodCaptureQueue struct {
	jobs    chan lodCaptureJob
	results chan lodCaptureJob
	pending map[chunkKey]uint64
	next    uint64
}

func (w *World) startLODObservedCapture() {
	if w.lodCapture != nil || !w.IsClient {
		return
	}
	c := &lodCaptureQueue{jobs: make(chan lodCaptureJob, 32), results: make(chan lodCaptureJob, 32), pending: make(map[chunkKey]uint64)}
	w.lodCapture = c
	go func() {
		defer close(c.results)
		for job := range c.jobs {
			job.record = buildLODObservedChunk(job.chunk, job.seed, job.key.X, job.key.Z)
			job.chunk = nil
			c.results <- job
		}
	}()
}

func (w *World) queueLODObservedChunk(chunk *Chunk, key chunkKey) bool {
	c := w.lodCapture
	if c == nil || len(c.jobs) == cap(c.jobs) {
		return false
	}
	private := &Chunk{blocks: chunk.blocks, heightMap: chunk.heightMap}
	for i := range private.blocks.sections {
		if data := private.blocks.sections[i].data; data != nil {
			copy := *data
			private.blocks.sections[i].data = &copy
		}
	}
	c.next++
	c.pending[key] = c.next
	if w.lodCacheEpoch == nil {
		w.lodCacheEpoch = make(map[chunkKey]uint64)
	}
	w.lodCacheEpoch[key]++
	c.jobs <- lodCaptureJob{key: key, seed: w.seed, id: c.next, chunk: private}
	return true
}

func (w *World) applyLODObservedResult(job lodCaptureJob) {
	if w.lodCapture.pending[job.key] != job.id || job.seed != w.seed {
		return
	}
	delete(w.lodCapture.pending, job.key)
	w.installLODObservedChunk(job.key, job.record, true)
}

func (w *World) processLODObservedResults() {
	if w.lodCapture == nil {
		return
	}
	for i := 0; i < 8; i++ {
		select {
		case job := <-w.lodCapture.results:
			w.applyLODObservedResult(job)
		default:
			return
		}
	}
}

func (w *World) finishLODObservedCapture() {
	if w.lodCapture == nil {
		return
	}
	close(w.lodCapture.jobs)
	// Drain while waiting, so a full result queue cannot deadlock shutdown.
	for job := range w.lodCapture.results {
		w.applyLODObservedResult(job)
	}
	w.lodCapture = nil
}

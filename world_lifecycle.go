package main

// The world owner (server loop or render thread) owns live chunks and queues.
// Workers receive private generated chunks or immutable mesh snapshots only.
type blockPos struct{ x, y, z int }
type blockEdit struct {
	pos   blockPos
	value byte
	meta  bool
}

func (w *World) deferEdit(cx, cz int, edit blockEdit) {
	if w.pendingEdits == nil {
		w.pendingEdits = make(map[chunkKey][]blockEdit)
	}
	key := chunkKey{cx, cz}
	w.pendingEdits[key] = append(w.pendingEdits[key], edit)
}

func (w *World) applyPendingEdits(key chunkKey) {
	edits := w.pendingEdits[key]
	delete(w.pendingEdits, key)
	for _, edit := range edits {
		p := edit.pos
		if edit.meta {
			w.SetMetaAt(p.x, p.y, p.z, edit.value)
		} else {
			w.SetBlockAt(p.x, p.y, p.z, edit.value)
		}
	}
}

// Close must run on the world owner, before unloading RenderAssets or the GL context.
// Cancellation also unblocks workers whose result queues are full.
func (w *World) Close() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		close(w.done)
		w.workers.Wait()
		for {
			select {
			case job := <-w.meshJobs:
				job.snapshot.Release()
			default:
				goto results
			}
		}
	results:
		for {
			select {
			case res := <-w.meshResults:
				releaseMeshResults(res.results)
			default:
				goto generated
			}
		}
	generated:
		for {
			select {
			case res := <-w.genResults:
				w.chunkPool.Put(res.chunk)
			default:
				goto chunks
			}
		}
	chunks:
		for key, chunk := range w.chunks {
			w.freeChunk(chunk)
			delete(w.chunks, key)
		}
		clear(w.pending)
		clear(w.immediate)
		clear(w.pendingEdits)
		clear(w.lightChanged)
		w.render = worldRenderCache{}
		w.entities = nil
	})
}

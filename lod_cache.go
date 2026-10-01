package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Bump when generator rules, block IDs, or observed-column interpretation change.
const lodCacheRevision = "terrain-v7-observed-v1"
const lodCacheMaxFiles = 4096
const lodCacheMaxFileBytes = 512 * 1024
const lodCacheMaxBytes = 64 << 20

type lodCacheJob struct {
	key    chunkKey
	record *lodObservedChunk
	epoch  uint64
}
type lodDiskCache struct {
	dir            string
	seed           uint32
	jobs           chan lodCacheJob
	loaded         chan lodCacheJob
	stopLoad       chan struct{}
	loader, writer sync.WaitGroup
	reads          chan lodCacheJob
	readPending    map[chunkKey]bool // World owner only.
	available      sync.Map          // Disk keys, shared with writer/loader.
	availableTiles sync.Map
	inFlight       sync.Map // Immutable records pending disk write; protects eviction.
	nextTrim       time.Time
	tilePoll       map[lodTileKey]time.Time
}

func lodSessionCacheIdentity(savePath, address string, multiplayer bool) string {
	if multiplayer {
		return "server:" + strings.ToLower(strings.TrimSpace(address))
	}
	path, err := filepath.Abs(savePath)
	if err != nil {
		path = filepath.Clean(savePath)
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return "save:" + lodLocalCacheIdentity(path)
}

func (w *World) openLODCache(root, identity string) {
	if !w.IsClient || identity == "" || w.lodCache != nil {
		return
	}
	namespace := sha256.Sum256([]byte(fmt.Sprintf("%s\n%d\n%s", identity, w.seed, lodCacheRevision)))
	dir := filepath.Join(root, fmt.Sprintf("%x", namespace))
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("LOD cache disabled: %v\n", err)
		return
	}
	c := &lodDiskCache{dir: dir, seed: w.seed, jobs: make(chan lodCacheJob, 8), loaded: make(chan lodCacheJob, 8), stopLoad: make(chan struct{}), reads: make(chan lodCacheJob, 32), readPending: make(map[chunkKey]bool)}
	w.lodCache = c
	c.writer.Add(1)
	go func() {
		defer c.writer.Done()
		writes := 0
		for job := range c.jobs {
			if err := c.write(job); err != nil {
				fmt.Printf("LOD cache write failed: %v\n", err)
			} else {
				c.available.Store(job.key, true)
				c.availableTiles.Store(lodTileKey{divFloor(job.key.X, 8), divFloor(job.key.Z, 8)}, true)
			}
			c.inFlight.CompareAndDelete(job.key, job.record)
			writes++
			if writes%64 == 0 {
				c.prune()
			}
		}
		c.prune()
	}()
	c.loader.Add(1)
	go func() {
		defer c.loader.Done()
		files := c.files()
		var bytesLoaded int64
		for i, entry := range files {
			if i >= lodCacheMaxFiles || bytesLoaded+entry.Size() > lodCacheMaxBytes {
				break
			}
			bytesLoaded += entry.Size()
			select {
			case <-c.stopLoad:
				return
			default:
			}
			// Index names only. The renderer requests nearby records first;
			// reopening a large cache must not deserialize the entire world.
			var key chunkKey
			if _, err := fmt.Sscanf(entry.Name(), "%d_%d.lod", &key.X, &key.Z); err == nil && entry.Name() == fmt.Sprintf("%d_%d.lod", key.X, key.Z) && entry.Size() <= lodCacheMaxFileBytes {
				c.available.Store(key, true)
				c.availableTiles.Store(lodTileKey{divFloor(key.X, 8), divFloor(key.Z, 8)}, true)
			}
		}
		for {
			select {
			case request := <-c.reads:
				job, err := c.read(filepath.Join(c.dir, fmt.Sprintf("%d_%d.lod", request.key.X, request.key.Z)))
				if err != nil {
					c.available.Delete(request.key)
					job = lodCacheJob{key: request.key}
				}
				job.epoch = request.epoch
				select {
				case c.loaded <- job:
				case <-c.stopLoad:
					return
				}
			case <-c.stopLoad:
				return
			}
		}
	}()
}

func (c *lodDiskCache) files() []os.FileInfo {
	entries, _ := os.ReadDir(c.dir)
	var files []os.FileInfo
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".lod") {
			if info, err := e.Info(); err == nil {
				files = append(files, info)
			}
		}
	}
	sort.Slice(files, func(i, j int) bool { return files[i].ModTime().After(files[j].ModTime()) })
	return files
}

func (c *lodDiskCache) prune() {
	files := c.files()
	var bytesKept int64
	for i, file := range files {
		if i < lodCacheMaxFiles && bytesKept+file.Size() <= lodCacheMaxBytes {
			bytesKept += file.Size()
			continue
		}
		_ = os.Remove(filepath.Join(c.dir, file.Name()))
	}
}

func (c *lodDiskCache) write(job lodCacheJob) error {
	var b bytes.Buffer
	b.WriteString("GCLD1")
	for _, v := range []any{c.seed, int64(job.key.X), int64(job.key.Z)} {
		if err := binary.Write(&b, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	for _, column := range job.record.columns {
		binary.Write(&b, binary.LittleEndian, uint16(column.height))
		b.WriteByte(column.top)
		if column.changed {
			b.WriteByte(1)
		} else {
			b.WriteByte(0)
		}
		binary.Write(&b, binary.LittleEndian, uint16(len(column.occupancy.spans)))
		for _, span := range column.occupancy.spans {
			binary.Write(&b, binary.LittleEndian, uint16(span.lo))
			binary.Write(&b, binary.LittleEndian, uint16(span.hi))
			b.WriteByte(span.block)
		}
	}
	name := filepath.Join(c.dir, fmt.Sprintf("%d_%d.lod", job.key.X, job.key.Z))
	temp, err := os.CreateTemp(c.dir, "lod-*.tmp")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if _, err = temp.Write(b.Bytes()); err != nil {
		temp.Close()
		return err
	}
	if err = temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, name)
}

func (c *lodDiskCache) read(path string) (lodCacheJob, error) {
	var result lodCacheJob
	info, err := os.Stat(path)
	if err != nil {
		return result, err
	}
	if info.Size() > lodCacheMaxFileBytes {
		return result, fmt.Errorf("oversized LOD cache")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return result, err
	}
	if len(data) < 25 || string(data[:5]) != "GCLD1" {
		return result, fmt.Errorf("invalid LOD cache header")
	}
	r := bytes.NewReader(data[5:])
	var seed uint32
	var x, z int64
	for _, v := range []any{&seed, &x, &z} {
		if err = binary.Read(r, binary.LittleEndian, v); err != nil {
			return result, err
		}
	}
	if seed != c.seed || int64(int(x)) != x || int64(int(z)) != z {
		return result, fmt.Errorf("LOD cache namespace mismatch")
	}
	result.key = chunkKey{int(x), int(z)}
	if filepath.Base(path) != fmt.Sprintf("%d_%d.lod", x, z) {
		return result, fmt.Errorf("LOD cache key mismatch")
	}
	result.record = &lodObservedChunk{}
	for i := range result.record.columns {
		var height, count uint16
		if err = binary.Read(r, binary.LittleEndian, &height); err != nil {
			return result, err
		}
		top, e1 := r.ReadByte()
		changed, e2 := r.ReadByte()
		if err = binary.Read(r, binary.LittleEndian, &count); err != nil {
			return result, err
		}
		if e1 != nil || e2 != nil || height > chunkHeight || changed > 1 || count > chunkHeight {
			return result, fmt.Errorf("invalid LOD column")
		}
		occupancy := &lodOccupancy{ground: int(height), surface: top}
		previous := int(height)
		for n := 0; n < int(count); n++ {
			var lo, hi uint16
			if err = binary.Read(r, binary.LittleEndian, &lo); err != nil {
				return result, err
			}
			if err = binary.Read(r, binary.LittleEndian, &hi); err != nil {
				return result, err
			}
			block, err := r.ReadByte()
			if err != nil {
				return result, err
			}
			if int(lo) < previous || hi <= lo || hi > chunkHeight || block == blockAir {
				return result, fmt.Errorf("invalid LOD span")
			}
			occupancy.spans = append(occupancy.spans, lodSpan{int(lo), int(hi), block})
			previous = int(hi)
		}
		result.record.columns[i] = lodColumn{height: int(height), top: top, occupancy: occupancy, observed: true, changed: changed == 1}
	}
	if r.Len() != 0 {
		return result, fmt.Errorf("trailing LOD cache data")
	}
	return result, nil
}

// Run on the world owner. Per-frame admission stays bounded; edits coalesce
// by chunk while disk is busy. Fresh server data always beats late cache loads.
func (w *World) processLODCache() {
	c := w.lodCache
	if c == nil {
		return
	}
	if now := time.Now(); !now.Before(c.nextTrim) {
		w.trimLODCacheMemory(lodCacheMaxFiles)
		c.nextTrim = now.Add(time.Second)
	}
	for i := 0; i < 4; i++ {
		select {
		case job := <-c.loaded:
			delete(c.readPending, job.key)
			if w.lodCapture != nil && w.lodCapture.pending[job.key] != 0 {
				continue
			}
			if job.record != nil && job.epoch == w.lodCacheEpoch[job.key] && w.lodObservedChunks[job.key] == nil {
				w.installLODObservedChunk(job.key, job.record, false)
			}
		default:
			i = 4
		}
	}
	submitted := 0
	for key := range w.lodCacheDirty {
		record := w.lodObservedChunks[key]
		c.inFlight.Store(key, record)
		select {
		case c.jobs <- lodCacheJob{key: key, record: record}:
			delete(w.lodCacheDirty, key)
			submitted++
		default:
			c.inFlight.CompareAndDelete(key, record)
			return
		}
		if submitted >= 4 {
			return
		}
	}
}

func (w *World) touchLODCache(key chunkKey) {
	if w.lodCacheAccess == nil {
		w.lodCacheAccess = make(map[chunkKey]uint64)
	}
	w.lodCacheClock++
	w.lodCacheAccess[key] = w.lodCacheClock
}

func (w *World) trimLODCacheMemory(limit int) {
	for attempts := 0; len(w.lodObservedChunks) > limit && attempts < 8; attempts++ {
		var oldest chunkKey
		stamp := ^uint64(0)
		found := false
		for key := range w.lodObservedChunks {
			if w.lodCacheDirty[key] || w.getChunkIfGenerated(key.X, key.Z) != nil {
				continue
			}
			if _, writing := w.lodCache.inFlight.Load(key); writing {
				continue
			}
			if age := w.lodCacheAccess[key]; !found || age < stamp {
				oldest, stamp, found = key, age, true
			}
		}
		if !found {
			return
		} // Live full chunks are governed by the render radius.
		delete(w.lodObservedChunks, oldest)
		delete(w.lodCacheAccess, oldest)
		for z := 0; z < chunkWidth; z++ {
			for x := 0; x < chunkWidth; x++ {
				point := lodPoint{oldest.X*chunkWidth + x, oldest.Z*chunkWidth + z}
				delete(w.lodColumns, point)
				key := lodTileKey{divFloor(point.X, lodTileSize), divFloor(point.Z, lodTileSize)}
				delete(w.lodOccupancyTiles[key], point)
			}
		}
	}
}

func (w *World) closeLODCache() {
	c := w.lodCache
	if c == nil {
		return
	}
	close(c.stopLoad)
	c.loader.Wait()
	for key := range w.lodCacheDirty {
		c.jobs <- lodCacheJob{key: key, record: w.lodObservedChunks[key]}
	}
	close(c.jobs)
	c.writer.Wait()
	w.lodCache = nil
	w.lodCacheDirty = nil
}

func (w *World) requestLODCacheChunk(key chunkKey) {
	c := w.lodCache
	if c == nil || c.readPending[key] {
		return
	}
	if _, known := c.available.Load(key); !known {
		return
	}
	select {
	case c.reads <- lodCacheJob{key: key, epoch: w.lodCacheEpoch[key]}:
		c.readPending[key] = true
	default:
	}
}

func (w *World) requestLODCacheTile(key lodTileKey) {
	c := w.lodCache
	if c == nil {
		return
	}
	if _, known := c.availableTiles.Load(key); !known {
		return
	}
	now := time.Now()
	if now.Before(c.tilePoll[key]) {
		return
	}
	if c.tilePoll == nil {
		c.tilePoll = make(map[lodTileKey]time.Time)
	}
	// Bound the scheduler metadata too when travelling through a large world.
	if len(c.tilePoll) > lodCacheMaxFiles {
		clear(c.tilePoll)
	}
	c.tilePoll[key] = now.Add(250 * time.Millisecond)
	for z := key.Z * 8; z < (key.Z+1)*8; z++ {
		for x := key.X * 8; x < (key.X+1)*8; x++ {
			chunk := chunkKey{x, z}
			if w.lodObservedChunks[chunk] != nil {
				w.touchLODCache(chunk)
			} else {
				w.requestLODCacheChunk(chunk)
			}
		}
	}
}

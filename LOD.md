# Distant terrain prototype

This branch implements a first geometry-LOD horizon inspired by Distant Horizons.
It is an independent prototype, not a port of that mod or a replacement for
full chunks. The normal render distance still controls real chunk requests,
meshing, collision, entities and interaction. A separate horizon radius draws
simplified, non-interactive terrain beyond it.

## Data and budget

- A 128×128-block LOD tile uses an 8-block grid (16×16 cells) sampled directly
  from the deterministic seed/terrain-column functions. It does not create or
  retain `Chunk` objects, compute caves, light propagation or entities. A
  cheap hash prefilter rejects most tree positions, then the shared tree-anchor
  generator supplies exact tree positions, species and heights. The distant
  trunk/crown meshes remain simplified representations of those trees.
- Land, biome surface colors, tree silhouettes and a flat water layer use
  compact WebGPU meshes. The distant ground remains beneath the full-detail
  radius as a lowered underlay, so lagging chunk delivery does not leave an
  empty ring. Approximate trees are hidden inside that radius; real chunks
  render over the underlay. A 128-block-wide height transition avoids a hard
  step where the full-detail radius ends.
- Two CPU workers generate tiles. A bounded queue and at most four GPU uploads
  per frame prevent a long single-frame stall. Visible tiles are requested
  nearest-first; meshes outside the radius are unloaded. GPU creation, upload
  and disposal remain on the render thread. Leaving the world releases its LOD
  meshes/workers even though the menu keeps the main renderer alive.
- Incoming client chunks contribute only the 8-block-grid surface samples that
  differ from the procedural seed. Decorations, natural tree leaves/logs and
  liquids are ignored for this comparison. Subsequent edits at sampled points
  refresh those columns; stale worker results are rejected by per-tile version
  numbers. A tile is rebuilt in the background while its previous mesh stays
  visible. Chunks first loaded while the horizon was disabled are sampled
  before unloading if the horizon has since been enabled. These sparse deltas
  survive ordinary chunk unloads but are cleared on world-seed change.
- The view projection and boundary fog follow the larger of full and distant
  radii. The settings page cycles the horizon through Off/64/96/128 chunks;
  default is 96, independently of the 24-chunk full-detail default.

`BenchmarkLODTileBuild` on an i7-14700HX measured roughly 1.65 ms per tile
with exact tree anchors (200 iterations; the prior approximate-tree version
measured about 1.39 ms and bare-ground about 0.70 ms). This is a CPU-only tile
microbenchmark, not a gameplay FPS or full-horizon completion measurement.

## Checks

- `go test . -run '^TestLODTile|^TestHorizonDistanceClamp'` verifies seed
  determinism, exact sampled heights/surface colors, negative coordinates,
  neighbor seams and index bounds.
- `GOCRAFT_WEBGPU_LOD_PREVIEW=1 go test . -run TestWebGPUDistantTerrainPreview -v`
  exercises native WebGPU gameplay, captures `work/lod-terrain.png`, and checks
  world-seed change and horizon-off cleanup.
- The existing native-window integration test covers two real TCP game
  sessions and returning to the menu.

## Deliberate limits before considering a main-branch merge

- Distant terrain is a sampled heightfield, not a full chunk or structure
  mesh. Server-sent edits at its grid points appear after the corresponding
  chunk has been received, but details between those points, underground
  caves, doors, vegetation edits and entities remain absent. Tree silhouettes
  occupy real generated tree positions but do not reproduce every leaf block.
  Natural logs/leaves are excluded from surface sampling, so log-only player
  structures are also not yet represented. Unvisited distant edits cannot be
  known without a separate server LOD protocol.
- One 8-block grid resolution is used throughout the far view. There is no
  multi-level quadtree, LOD disk cache or distant structure representation.
- Biome colors are a small palette rather than averaged atlas textures. Water
  is an opaque, flat color, and transitions at shorelines/near-full meshes
  still need visual inspection in live play. The offscreen preview deliberately
  contains no full chunks, so it verifies the fallback underlay but cannot
  prove a seamless real-chunk handoff.
- Full `RenderDistance=128` still requests complete chunks. For the intended
  experiment, keep full distance around 16–32 and use `Horizon=96/128`.

The next version should prioritize overlap/occlusion checks in live terrain,
then synchronize saved edits and add at least one coarser outer LOD level.

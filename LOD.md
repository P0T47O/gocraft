# Distant terrain prototype

This branch implements a first geometry-LOD horizon inspired by Distant Horizons.
It is an independent prototype, not a port of that mod or a replacement for
full chunks. The normal render distance still controls real chunk requests,
meshing, collision, entities and interaction. A separate horizon radius draws
simplified, non-interactive terrain beyond it.

## Data and budget

- A 128×128-block LOD tile uses an 8-block grid (16×16 cells) sampled directly
  from the deterministic seed/terrain-column functions. It does not create or
  retain `Chunk` objects, compute caves, light propagation, foliage or entities.
- Land, biome surface colors and a flat water layer use compact WebGPU meshes.
  Real chunks render over the distant surface; the LOD shader clips an inner
  circle two chunks inside the full-detail radius. This allows a short overlap
  while the normal mesh stream catches up.
- Two CPU workers generate tiles. A bounded queue and at most four GPU uploads
  per frame prevent a long single-frame stall. Visible tiles are requested
  nearest-first; meshes outside the radius are unloaded. GPU creation, upload
  and disposal remain on the render thread. Leaving the world releases its LOD
  meshes/workers even though the menu keeps the main renderer alive.
- The view projection and boundary fog follow the larger of full and distant
  radii. The settings page cycles the horizon through Off/64/96/128 chunks;
  default is 96, independently of the 24-chunk full-detail default.

`BenchmarkLODTileBuild` on an i7-14700HX measured roughly 0.70 ms and 60 KiB
of temporary allocations per tile (two short runs). This is a CPU-only tile
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

- Distant terrain comes from the procedural seed, not persisted/edited chunk
  snapshots. Player-built structures, mining, caves, trees and entity motion
  are absent; edits outside the full-detail radius will not appear until their
  real chunks load. Multiplayer therefore has no authoritative LOD updates yet.
- One 8-block grid resolution is used throughout the far view. There is no
  multi-level quadtree, LOD disk cache or distant structure representation.
- Biome colors are a small palette rather than averaged atlas textures. Water
  is an opaque, flat color, and transitions at shorelines/near-full meshes
  still need visual inspection in live play. The offscreen preview deliberately
  contains no full chunks, so its inner clipped circle is a test fixture, not
  a claimed final gameplay image.
- Full `RenderDistance=128` still requests complete chunks. For the intended
  experiment, keep full distance around 16–32 and use `Horizon=96/128`.

The next version should prioritize overlap/occlusion checks in live terrain,
then synchronize saved edits and add at least one coarser outer LOD level.

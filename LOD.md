# Distant terrain prototype

This branch implements a first geometry-LOD horizon inspired by Distant Horizons.
It is an independent prototype, not a port of that mod or a replacement for
full chunks. The normal render distance still controls real chunk requests,
meshing, collision, entities and interaction. A separate horizon radius draws
simplified, non-interactive terrain beyond it.

## Data and budget

- A 128×128-block LOD tile uses a 4-, 8-, or 16-block grid, chosen by distance
  from the camera beyond the full-chunk radius. The first eight chunks beyond
  full detail use 4-block cells, the next 24 use 8-block cells, and the outer
  horizon uses 16-block cells. A one-chunk hysteresis band prevents repeated
  rebuilds while hovering near a level boundary. The 4- and 8-block terrain
  vertices also carry the height of their next-coarser triangulation. During
  the last 112 blocks before a level boundary, the GPU gradually morphs each
  tile toward that height; the two levels match when the mesh is swapped.
  Tiles are sampled directly
  from the deterministic seed/terrain-column functions. They do not create or
  retain `Chunk` objects, compute caves, light propagation or entities. A
  cheap hash prefilter rejects most tree positions, then the shared tree-anchor
  generator supplies exact tree positions, species and heights. The distant
  trunk/crown meshes remain simplified representations of those trees.
- Land, biome surface colors, tree silhouettes and a flat water layer use
  compact WebGPU meshes. The distant ground remains beneath the full-detail
  radius as a lowered underlay, so lagging chunk delivery does not leave an
  empty ring. Approximate trees are hidden inside that radius; real chunks
  render over the underlay. A 128-block-wide height transition avoids a hard
  step where the full-detail radius ends. Simplified tree silhouettes gain
  coverage gradually after the full radius, and a subtle block-scale color
  variation fades with distance. Adjacent levels share an interpolated
  16-block edge profile, so different cell sizes do not leave open seams. The
  morph uses tile-center distance, matching the CPU level selector.
- Two CPU workers generate tiles. A bounded queue and at most four GPU uploads
  per frame prevent a long single-frame stall. Visible tiles are requested
  nearest-first; meshes outside the radius are unloaded. GPU creation, upload
  and disposal remain on the render thread. Leaving the world releases its LOD
  meshes/workers even though the menu keeps the main renderer alive.
- Incoming client chunks contribute only the 8-block-grid surface samples that
  differ from the procedural seed. Decorations, natural tree leaves/logs and
  liquids are ignored for this comparison. Subsequent edits at sampled points
  refresh those columns; stale worker results are rejected by per-tile version
  and detail level. A tile is rebuilt in the background while its previous mesh
  stays visible. Chunks first loaded while the horizon was disabled are sampled
  before unloading if the horizon has since been enabled. These sparse deltas
  survive ordinary chunk unloads but are cleared on world-seed change.
- The view projection and boundary fog follow the larger of full and distant
  radii. The settings page cycles the horizon through Off/64/96/128 chunks;
  default is 96, independently of the 24-chunk full-detail default.

`BenchmarkLODTileLevels` on an i7-14700HX measured roughly 4.10 / 1.87 /
1.21 ms per 4 / 8 / 16-block tile. The finer level is restricted to the
inner band; the outer horizon is cheaper than the former fixed 8-block grid.
This is a CPU-only tile microbenchmark, not a gameplay FPS or full-horizon
completion measurement.

## Checks

- `go test . -run '^TestLOD|^TestHorizonDistanceClamp'` verifies seed
  determinism, sampled heights/surface colors, negative coordinates, mixed
  4/16-level neighbor seams, parent-level morph heights, level hysteresis and
  index bounds.
- `GOCRAFT_WEBGPU_LOD_PREVIEW=1 go test . -run TestWebGPUDistantTerrainPreview -v`
  exercises native WebGPU gameplay, captures `work/lod-terrain.png`, and checks
  world-seed change and horizon-off cleanup.
- `GOCRAFT_WEBGPU_STRUCTURE_PREVIEW=1 go test . -run TestWebGPUDistantStructuresAtThreeDistances -v`
  builds a diagnostic 64×64 cuboid, square pyramid, and horizontal ring from
  captured surface columns, then captures `work/lod-structures-{near,middle,far}.png`
  on native WebGPU at 4/8/16-block detail. The fixture does not place whole
  voxel buildings into a save; it supplies the same column summaries that a
  received, edited client chunk would provide. The CPU test also verifies
  all three shapes' sampled top blocks and that an unvisited structure remains
  unknown to the distant renderer.
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
- The three fixed-distance bands are not a multi-level quadtree. Terrain
  heights now morph between adjacent grid levels, but color variation, water
  cell boundaries and simplified tree silhouettes can still change at a mesh
  swap. There is no LOD disk cache or distant structure representation.
- Sparse authoritative edits are sampled on the 8-block lattice. Edits lying
  between 16-block outer vertices or between 4-block inner vertices are not
  fully represented. Coarse water-cell classifications can also differ at a
  mixed-level shoreline even though the ground edges share a height profile.
- The three-building preview exposes a larger architectural limit: the cuboid
  acquires sloped skirts and the ring looks like a solid mound or wall from a
  side view. A single top height and color per column cannot preserve vertical
  building walls, overhangs or a vertical ring's opening. The 4-block level
  can make these slopes especially sharp. Geometry morphing does not fix this;
  a separate distant structure/occupancy representation is needed. Before a
  chunk has ever been received, even a giant player-built shape is invisible
  because the client has no authoritative distant structure data.
- Biome colors are a small palette rather than averaged atlas textures. Water
  is an opaque, flat color, and transitions at shorelines/near-full meshes
  still need visual inspection in live play. The offscreen preview deliberately
  contains no full chunks, so it verifies the fallback underlay but cannot
  prove a seamless real-chunk handoff.
- Full `RenderDistance=128` still requests complete chunks. For the intended
  experiment, keep full distance around 16–32 and use `Horizon=96/128`.

The next version should prioritize a live full-chunk/LOD overlap screenshot,
then a terrain-texture and water-shore strategy if the handoff remains visibly
abrupt, followed by persistent edit/structure synchronization.

# Distant terrain prototype

This branch implements a first geometry-LOD horizon inspired by Distant Horizons.
It is an independent prototype, not a port of that mod or a replacement for
full chunks. The normal render distance still controls real chunk requests,
meshing, collision, entities and interaction. A separate horizon radius draws
simplified, non-interactive terrain beyond it.

The transition design was compared with the public Distant Horizons rendering
approach at https://gitlab.com/distant-horizons-team/distant-horizons . This
implementation is original Go/WebGPU code; no Distant Horizons source or assets
were copied.

## Data and budget

- A 128×128-block LOD tile uses a 2-, 4-, 8-, or 16-block grid, chosen by distance
  from the camera beyond the full-chunk radius. The first four chunks beyond
  full detail use 2-block cells, the next eight use 4-block cells, the next 20
  use 8-block cells, and the outer horizon uses 16-block cells. A one-chunk
  hysteresis band prevents repeated rebuilds near a level boundary. The 2-, 4-
  and 8-block terrain
  vertices also carry the height of their next-coarser triangulation. Before
  each level boundary, the GPU gradually morphs tiles toward that height;
  the first transition uses a shorter band to retain 2-block detail at the
  full-chunk edge.
  Missing chunks within twelve chunks of the camera use 2-block fallback;
  other interior fallback tiles use 4-block cells.
  Tiles are sampled directly from the deterministic seed/terrain-column
  functions. They do not create or retain `Chunk` objects, compute caves, light
  propagation or entities. A
  cheap hash prefilter rejects most tree positions, then the shared tree-anchor
  generator supplies exact tree positions, species and heights. The distant
  trunk/crown meshes remain simplified representations of those trees.
- Land, climate-tinted grass, tree silhouettes and water use compact WebGPU
  meshes. Non-grass material colors use the alpha-weighted average of their
  loaded top-face texture; the old palette remains a fallback without an atlas.
  This removes the hard-coded gray snow mismatch and follows resource changes.
  Water now has a separate blended pass using the same atlas tile and
  biome tint and transparency as full-detail water, revealing the sampled
  terrain bed beneath. Ground and water use their normal surface heights;
  the old radial height offset and water fade created a visible ledge once the
  per-chunk mask exposed the LOD boundary. Distant ground remains available
  inside the full-detail radius while full chunks load. A camera-centered
  258×258-chunk mask hides distant terrain and water inside the real draw
  radius once all occupied surface sections of a chunk have meshes. It keeps
  real ownership after later edits, so mining does not expose the coarse
  surface. Cached chunks outside the draw radius and chunks still loading
  retain the fallback. The mask also switches between real and
  simplified trees at the chunk edge; the former distance fade left a bare
  forest ring. A subtle block-scale color
  variation fades with distance. Adjacent levels share an interpolated
  16-block edge profile, so different cell sizes do not leave open seams. The
  morph uses tile-center distance, matching the CPU level selector. Terrain
  and water grid positions use the same half-block footprint as real voxels.
  At an actual loaded-chunk boundary, the first eight LOD blocks use a
  replacement strip that starts at the adjacent real terrain column and
  tapers back to the ordinary LOD height and color. All four boundary
  directions are supported; coarse distant tiles omit unused strips. The
  strip now samples the real edge at every voxel even when its interior grid
  uses 2- or 4-block cells. Its inner lip extends slightly beneath the real
  voxel surface, and the ordinary LOD ground remains beneath the strip; this
  fills subpixel holes at the handoff. When an
  adjacent real chunk has not arrived, its mesh snapshot samples procedural
  water or ice for the one-column border instead of treating it as air; this
  suppresses temporary vertical water walls. A received neighbor, including
  player edits, overrides that procedural border.
- Two CPU workers generate tiles. A bounded queue and at most four GPU uploads
  per frame prevent a long single-frame stall. Visible tiles are requested
  nearest-first; meshes outside the radius are unloaded. GPU creation, upload
  and disposal remain on the render thread. Leaving the world releases its LOD
  meshes/workers even though the menu keeps the main renderer alive.
- Incoming client chunks contribute only the 8-block-grid surface samples that
  differ from the procedural seed. Decorations, natural tree leaves/logs and
  liquids are ignored for this comparison. Samples raised above their
  procedural ground height now form separate sparse 8×8 prisms: a top face and
  exposed vertical sides. The ground beneath uses its original height, so
  artificial buildings do not become terrain spikes. A one-sample snapshot
  halo supplies the neighbor heights needed for walls across tile borders.
  Subsequent edits at sampled points refresh those columns; stale worker
  results are rejected by per-tile version and detail level. A tile is rebuilt
  in the background while its previous mesh stays visible. Chunks first loaded
  while the horizon was disabled are sampled
  before unloading if the horizon has since been enabled. These sparse deltas
  survive ordinary chunk unloads but are cleared on world-seed change.
- The view projection and boundary fog follow the larger of full and distant
  radii. The settings page cycles the horizon through Off/64/96/128 chunks;
  default is 96, independently of the 24-chunk full-detail default.

`BenchmarkLODTileLevels` on an i7-14700HX measured roughly 4.10 / 1.87 /
1.21 ms per 4 / 8 / 16-block tile. The finer level is restricted to the
inner band; the outer horizon is cheaper than the former fixed 8-block grid.
This is a CPU-only tile microbenchmark, not a gameplay FPS or full-horizon
completion measurement. `BenchmarkLODStructureTile` with a pyramid and half a
ring measured roughly 4.27 / 2.09 / 1.41 ms after deduplicating neighboring
column lookups. Structure geometry has a local per-tile cost, but empty tiles
skip it entirely.

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
  plus `work/lod-structures-ring-overhead.png` on native WebGPU at 4/8/16-block
  detail. The fixture does not place whole
  voxel buildings into a save; it supplies the same column summaries that a
  received, edited client chunk would provide. The CPU test also verifies
  all three shapes' sampled top blocks and that an unvisited structure remains
  unknown to the distant renderer.
- `GOCRAFT_WEBGPU_STRUCTURE_PREVIEW=1 go test . -run TestWebGPUDistantVerticalAndFloatingStructures -v`
  creates a real-voxel vertical obsidian ring (empty center) and an unsupported
  iron platform (air beneath) in temporary in-memory chunks. Their captured
  columns are rendered at 4/8/16-block detail and saved as
  `work/lod-vertical-{near,middle,far}.png`. The result confirms that both
  collapse into ground-connected solid prisms at every distance. This test
  documents the current failure; passing it does **not** mean those two shapes
  render correctly.
- The existing native-window integration test covers two real TCP game
  sessions and returning to the menu.

## Deliberate limits before considering a main-branch merge

- Distant terrain remains a sampled heightfield with a sparse 2.5D prism layer,
  not a full chunk or voxel-accurate structure mesh. Server-sent edits at its
  grid points appear after the corresponding
  chunk has been received, but details between those points, underground
  caves, doors, vegetation edits and entities remain absent. Tree silhouettes
  occupy real generated tree positions but do not reproduce every leaf block.
  Natural logs/leaves are excluded from surface sampling, so log-only player
  structures are also not yet represented. Unvisited distant edits cannot be
  known without a separate server LOD protocol.
- The four fixed-distance bands are not a multi-level quadtree. Terrain
  heights now morph between adjacent grid levels, but color variation, water
  cell boundaries and simplified tree silhouettes can still change at a mesh
  swap. There is no LOD disk cache or underground occupancy representation.
- Sparse authoritative edits are sampled on the 8-block lattice. Edits lying
  between 16-block outer vertices or between 4-block inner vertices are not
  fully represented. Coarse water-cell classifications can also differ at a
  mixed-level shoreline even though the ground edges share a height profile.
- Received chunks now retain every above-ground structure column as immutable
  material/Y runs. All block-edit coordinates update these records, which
  survive full-chunk unloading. Logs matching procedural tree trunks are
  excluded; other log geometry is retained. Exposed top/bottom/side surfaces
  preserve vertical holes and suspended platforms at every terrain LOD level.
  The real packet/edit/unload/reload/demolition regression covers an off-grid
  wooden roof; the GTX 1060 native occupancy preview shows a hollow vertical
  ring and suspended platform. The legacy 8×8 prism diagnostics still document
  the old top-only representation; live captured structures use voxel runs.
  Underground structures, partial-block shapes, per-face textures and distant
  simplification of dense buildings remain unfinished. Structure-specific
  overlap with simultaneously rendered full chunks still needs live inspection.
- Before a chunk has been received in the current session, player buildings
  remain unknown. Restarting the game clears the in-memory summaries; receiving
  the saved chunk restores them. Persistent server summaries are still needed
  for unseen distant buildings after restarting.
- Terrain uses texture averages rather than full textured surfaces; near-field
  AO and voxel-side shading are still absent. Water is textured and blended, but transitions at shorelines
  and near-full meshes still need visual inspection in live play. The distant
  terrain preview has no full chunks; the separate handoff preview draws both
  renderers with the actual chunk mask. Edited edge columns and complex
  overhangs can still differ from the seed-based transition strip.
- The handoff strips use procedural columns except at stored 8-block edit
  samples. Between samples, a player-edited real edge can still differ from
  the LOD edge. Continuous sculpted geometry needs finer authoritative edge
  data; four-direction strips only remove the directional omission.
- Full `RenderDistance=128` still requests complete chunks. For the intended
  experiment, keep full distance around 16–32 and use `Horizon=96/128`.

The next version should prioritize a live full-chunk/LOD overlap screenshot,
then a terrain-texture and water-shore strategy if the handoff remains visibly
abrupt, followed by persistent edit/structure synchronization.

See [LOD_IMPROVEMENTS.md](LOD_IMPROVEMENTS.md) for the current distance,
snow-material and authoritative building-summary priorities.

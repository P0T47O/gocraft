# World generation

## Classic biome baseline

The current generator is inspired by the Java release-1.0 era, not a port of
Minecraft's algorithm or a seed-compatible implementation. It generates plains,
mixed oak/birch forest, desert, snowy spruce taiga, snowy plains, and extreme
hills, with ocean/frozen ocean and sandy/snowy shoreline transitions. Older
experimental biome constants remain internal but are not selected by this path.

`generation_biomes.go` now implements a continuous region model, replacing the
earlier threshold-and-height-offset generator. Salted gradient noise determines
climate, continentalness and mountain regions. Six nonnegative weights sum to
one; the dominant biome ID is only a label. Each region has its own height
profile (rolling plains/forest, dunes, cold hills, and ridged mountains), blended
before voxel quantization. Mountain foothills follow broad region weights while
ridge and valley fields shape the interior; no clamped mountain height offset.

Surface materials use these weights plus correlated 19-block patches. Desert
and snow boundaries can contain both materials rather than changing at the
biome label. Central differences of unquantized height determine slope. Rock
exposure follows slope and mountain weight, with no fixed Y=94 cutoff; high
gentle ground stays vegetated. Deserts have sand over sandstone.
Cold land uses full snow blocks (thin snow layers are not implemented), and
cold coastal water freezes. Forests mix oak with occasional birch; taiga uses
spruce; desert and snowy-plain interiors are treeless, while mixed edges may
carry neighboring vegetation. Tree density fades using the same region weights
and slope. Foliage/water tint also blends weights. Grass and flowers use
spatial patches, while desert decoration uses cacti and dead bushes.

This is a first subset: no swamp, mushroom island, hydrological river network, structures,
thin snow, or new biome-specific animal spawn rules are implemented here.
The heightfield does not reproduce original extreme-hills overhangs. Existing
caves and ore generation are unchanged. Tree crowns still use simple templates.

### River channels

Rivers carve the already-blended heightfield along a warped gradient-noise
contour at a 900-block scale. Gradient normalization approximates distance in
blocks; a separate noise varies channel width. Broad valley shoulders increase
with terrain height, while squared blending preserves the outer foothills.
The channel bed varies around Y=58, and water uses the existing Y=61 surface.
A separate low, irregular shelf separates the waterline from the outer valley;
signed contour distance and a broad noise vary the width on each bank separately.
This replaces the earlier symmetric single-curve bowl. Ordinary
riverbeds use gravel over dirt; desert beds use sand, and cold channels freeze.
The ocean/shore labels remain distinct from inland carved river labels. Trees
cannot anchor below sea level; slope and surface rules govern banks.

This is a deterministic channel feature, NOT drainage simulation: contours can
form loops or inland closed segments; there is no guarantee every channel
reaches an ocean, no flow direction, and no altitude-varying water surface.
Existing diagnostic maps and `TestGenerationRiversAndFrozenWater` exercise the
real sampler, including river water/ice and chunk/procedural agreement.

No old-generator compatibility is maintained during this development phase.
Create a new world to inspect the new generator. Existing saves are not deleted
or migrated automatically.

`TestGenerationClassicBiomeCoverage` samples five seeds on a 64-block grid over
[-4096,4096] in X/Z, checks every selected biome, surfaces, ice, classification
agreement, sampled adjacent-column continuity and mountain/plains separation.
These distribution checks are fixtures, not claims about every possible seed.

Newly generated chunks now use a shared terrain column sample for terrain height,
surface/filler materials, sea filling, ice, and procedural terrain queries.
Height means the first voxel above the solid surface; solid terrain ends at
height minus one. The bounded profiles leave building headroom, preserving
bedrock at Y=0. Water fills up to Y=61 wherever terrain is below sea level.
Procedural queries outside the world height return air.

The old height spline and duplicate biome classifier have been removed. Legacy
value noise remains for some decoration rolls, not the new terrain shape.

Trees use deterministic world-coordinate anchors, including a two-block halo
outside each chunk. Anchors use the terrain surface, unaffected by earlier trees.
Each intersecting tree is replayed in ascending world X/Z order. Logs take
precedence over leaves, and the first anchor wins equal-priority overlaps.
Only air/leaves can be replaced; trees do not overwrite hills or water.
Flower patches are tested before tall grass, making both flower types reachable.

Caves intersect two smooth, continuous 3D world-coordinate noise fields. They
preserve Y=0 through Y=3, the upper eight terrain layers, all ocean columns, and
all columns below sea level. They remain sealed underground and do not generate
surface entrances. Ore placement uses an ordered 64-bit coordinate hash instead
of overlapping shifted coordinate fields. Ore veins remain chunk-local.

## Existing saves and limits

Saved chunks are still loaded before generation; these changes do not migrate,
rewrite, or regenerate existing saved terrain or player edits. Only chunks that
need generation use the new algorithm. The same seed therefore produces
different new caves, ore placement, trees, and vegetation than older versions.
Transitions to old saved chunks can show differences, including incomplete
trees at old/new boundaries; existing chunks are intentionally not repaired.
There is no generator-version selector or old-world reproduction mode.

Procedural fallback matches base terrain, surface materials, water/ice, and
caves. It intentionally excludes ores, trees, cacti, flowers, and other
decorations, which appear when chunks are generated. It cannot represent saved
player edits. The existing terrain/biome noise still uses float32 coordinates,
so precision at extremely distant coordinates remains limited.

## Headless verification

Optional reproducible diagnostic maps, using the real production sampler:

```powershell
$env:GOCRAFT_REVIEW_DIR = 'work/generation-review'
go test -run TestGenerationReviewMaps -v .
Remove-Item Env:GOCRAFT_REVIEW_DIR
```

Exports surface, hillshaded height and slope PNGs for seed 42, selecting a
strong inland mountain and sampling a 1536-block region. These are CPU maps,
not in-game screenshots. Transition tests check mixed sand/grass and exposed
rock alongside high grass. Visual review remains necessary in addition to tests.

With Go 1.21 or newer, run from the repository root:

```text
go run ./tests/generation
```

The harness extracts the actual generation declarations and block constants,
copies the generation source and tests into a temporary module, and runs the
tests against minimal chunk storage. It requires only the standard library and
does not link graphics, workers, or persistence. It removes its temporary
directory when finished. Set GOROOT when invoking a Go executable outside PATH.

The same tests can run with the integrated application:

```text
go test -run TestGeneration -count=1 -v .
```

Tests cover seed/reuse determinism, generated/procedural terrain and cave
agreement, exact surface/sea bounds and height maps, both flower rolls, cave
protection and continuity, coordinate-hash aliases, and overlapping trees
clipped across chunk boundaries against a larger world-coordinate reference.
Persistence and asynchronous worker behavior are outside this suite.

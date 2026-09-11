# World generation

Newly generated chunks now use a shared terrain column sample for terrain height,
surface/filler materials, sea filling, ice, and procedural terrain queries.
Height means the first voxel above the solid surface; solid terrain ends at
height minus one. Height is clamped to 2 through chunkHeight minus one, preserving
bedrock at Y=0. Water fills up to Y=61 wherever terrain is below sea level.
Procedural queries outside the world height return air.

Trees use deterministic world-coordinate anchors, including a two-block halo
outside each chunk. Anchors use the terrain surface, unaffected by earlier trees.
Each intersecting tree is replayed in ascending world X/Z order. Logs take
precedence over leaves, and the first anchor wins equal-priority overlaps.
Only air/leaves can be replaced; trees do not overwrite hills or water.
Flowers are tested before tall grass, making both roses and dandelions reachable.

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

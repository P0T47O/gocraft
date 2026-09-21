# WebGPU-only migration

## Current state

Raylib has been removed from Go imports, go.mod/go.sum and the build/test dependency graph. Windows clients now use the native Win32 window and WebGPU exclusively. The old OpenGL renderer, implicit OpenGL mesh backend, Raylib resource types, UI adapters and old gameplay loop were deleted. The historical Raylib license is retained for provenance, not as an active dependency.

- Default: `go run .` (Windows). `-webgpu` remains accepted; `-webgpu=false` reports that the legacy renderer was removed.
- Dedicated server: `go run . -server`.
- Model workshop: `go run . -mob-preview`, native input and GPU, no saves/network. 1–5 selects state, A/D rotates, B toggles collision bounds, R reloads JSON content and rebuilds the atlas.
- `go run ./cmd/webgpu-smoke` is now a source-tree launcher for the native real-chunk preview. It requires Go and no longer uses a separate Raylib cube renderer.
- Game camera, input, physics, saves, CPU atlas UVs and mesh construction contain no window-library resource types.
- GPU allocations and release stay on the rendering thread. World meshes are released before their device; the HWND outlives presentation resources.
- No implicit backend after device shutdown: an attempted mesh upload without an installed renderer fails explicitly rather than entering OpenGL.

## Regression coverage

Normal validation: `go test ./...`, `go vet ./...`, `go build .`, `go run ./tools/codeindex -check`.
Dependency guard: `TestRepositoryHasNoRaylibDependency` scans Go imports and module files. Also inspect `go list -deps -test ./...` when dependencies change.

Set `GOCRAFT_WEBGPU_REGRESSION=1` for native window/input, resolution recovery, temporary-world rejoin, model workshop, mipmap movement/animation and GPU upload-isolation tests.

Existing visual preview switches remain available:
- `GOCRAFT_MENU_PREVIEW`, `GOCRAFT_INVENTORY_PREVIEW`, `GOCRAFT_CONTAINER_PREVIEW`, `GOCRAFT_VITALS_PREVIEW`, `GOCRAFT_MOB_PREVIEW`, `GOCRAFT_LIGHT_PREVIEW`: set to 1; screenshots go to ignored work/.
- `GOCRAFT_FILTER_GPU_TEST=1`: now runs WebGPU sampler/UI and mipmap pixel checks instead of querying OpenGL state.
- Terrain previews: `GOCRAFT_WEBGPU_CHUNK_PREVIEW`, `GOCRAFT_WEBGPU_REGION_PREVIEW`, `GOCRAFT_WEBGPU_SURFACE_PREVIEW`, `GOCRAFT_WEBGPU_TEXTURE_PREVIEW`, `GOCRAFT_WEBGPU_TRANSPARENCY_PREVIEW`.
- With a terrain preview enabled, `GOCRAFT_WEBGPU_PREVIEW_FRAMES=3` makes it hidden, resizes on the second frame and exits after three. Otherwise it is interactive.

Menu/inventory/container/vitals screenshots use the production GPU UI batches, not a separate reference renderer. Mob images exercise current CPU pose evaluation and GPU geometry. The smooth-light comparison asserts flat reference pixels versus interpolated lighting. Respawn retry tests use the native input frame clock, replacing the last Raylib timer call.

## Remaining limits

The native client/window currently supports Windows only. The built-in font is ASCII-only; IME composition and real cross-monitor DPI interactions still need manual validation. Native regression tests do not constitute a long gameplay soak test. Terrain generation and save formats were not changed by this removal.

The maintained feature map is [CODE_INDEX.md](CODE_INDEX.md); older implementations can be recovered from Git history.

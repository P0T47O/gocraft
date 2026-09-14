# GoCraft

![GoCraft Logo](assets/logo.png)

[English](README.md) | [中文](README_zh.md)

> [!IMPORTANT]
> **Project Status: Alpha**. GoCraft is an experimental high-performance voxel engine and survival sandbox written in Go and Raylib. It features an authoritative multiplayer architecture, deterministic procedural terrain, survival progression, container processing, and data-driven entities.

**GoCraft** is a voxel engine experiment exploring clean architecture, procedural terrain generation, custom networking, and low-level graphics optimization in Go.

![GoCraft](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-GPLv3-blue.svg)

---

## Features

- **Procedural World & Classic Biomes**:
  - Continuous 6-region climate model: Plains, mixed Oak/Birch Forest, Desert, Spruce Taiga, Mountains (Extreme Hills), Snowy Plains, Oceans, Frozen Oceans, and Beaches.
  - Deterministic 900-block scale river channels and frozen rivers carved directly into the terrain with gravel and sand riverbeds.
  - Smooth bilinear transitions for grass, foliage, water tints, and distance fog.
  - Slope-aware rock exposure, continuous underground cave networks, and deterministic world-coordinate vegetation placement.
- **Rich Survival Gameplay**:
  - Full survival inventory and crafting with dynamic recipe book, search/filter, and automatic wood-variant recipe substitution.
  - Centralized mining profiles with explicit harvest eligibility, tool tiers, and usage-based durability degradation.
  - Persistent 27-slot chests and fuel-burning furnaces with authoritative server sessions.
  - Player health (20 HP), underwater air (15s), fall/fire/lava/drowning/void damage, death drops, and safe spawn anchor recovery.
- **Data-Driven Entities & Mobs**:
  - Extensible JSON-defined actor models, collision geometry, stride animations, soundless behavior AI (wander, stop, flee), and combat.
  - Integrated visual model workshop (`./gocraft.exe -mob-preview`) with real-time file reloading.
- **Optimized UI & Pixel Theme**:
  - Consistent dark pixel theme across menus, containers, recipe lists, and HUD hotbar.
  - Persistent configuration (`settings.json`) supporting 16:9, 16:10, 21:9, and 4:3 display aspect ratios with adjustable mouse sensitivity.
- **Authoritative Multiplayer**:
  - Dedicated TCP server/client architecture with entity interpolation, transaction-verified container interactions, and chat commands.
  - Resilient atomic file persistence (`.tmp` + rename) protecting level metadata, player state, containers, and compressed chunk storage against unexpected crashes.
- **Advanced Lighting & Shaders**:
  - Propagated sky and block lighting with ambient occlusion, day/night cycles, and custom GLSL cutout/fog shaders.

---

## 🚧 Current Development Status (WIP)

- **Audio**: Sound effects and background music are yet to be integrated.
- **Food & Hunger**: Health regenerates slowly over time out of danger; porkchops can be smelted/cooked, but full hunger bars and eating mechanics are in progress.
- **Mob Expansion**: Pig actor model, movement physics, animations, and combat are complete; additional hostile/passive mobs, breeding, and advanced A* pathfinding are planned.
- **Multiplayer Mining Authority**: Break timing is currently client-side with server harvest validation; fully server-authoritative mining sessions remain future work.

---

## Getting Started

### Prerequisites

- [Go](https://go.dev/dl/) 1.21 or higher.
- A C compiler (GCC/MinGW) is required for cgo (used by raylib-go).

### Running the Game

1. **Clone the repository:**
   ```bash
   git clone https://github.com/P0T47O/gocraft.git
   cd gocraft
   ```

2. **Run the Client (Singleplayer / Default):**
   ```bash
   go run .
   ```

   **Note on Resource Packs**: 
   This repository does not include copyrighted game assets. The game runs with procedural checkerboard placeholders by default. To use a standard resource pack:
   1. Locate or create a `textures/` folder in the project root directory.
   2. Use any standard Minecraft Java Edition (1.20+) resource pack or client jar.
   3. Open the `.zip` / `.jar`, navigate to `assets/minecraft/textures/`.
   4. Extract its contents (`block`, `item`, `gui`, etc.) into your local `textures/` folder.
   
   *(The `textures/` and `minecraft/` folders are already configured in `.gitignore` to prevent committing copyrighted files).*

### Multiplayer

To start a dedicated server:
```bash
go run . -server
```

To join a server as a specific player:
```bash
go run . -name PlayerName
```

---

## Controls

| Key | Action |
|---|---|
| **W, A, S, D** | Move |
| **Space** | Jump / Swim Up / Fly Up |
| **Left Control + W** / **Double-tap W** | Sprint |
| **Shift** | Sneak at edges / Swim Down / Fly Down |
| **Left Click** | Attack Mob / Break Block |
| **Right Click** | Place Block / Open Container / Use Workbench |
| **1 – 9** | Select Hotbar Slot |
| **E** | Open / Close Inventory & Recipe Book |
| **Q** | Drop Selected Item (preserves durability) |
| **F1** | Toggle Survival / Creative Mode |
| **F** | Toggle Flying Mode (Creative) |
| **F3** | Toggle Debug Overlay |
| **ESC** | Open / Close Pause Menu |

---

## Deep Dive: Systems & Architecture

### Survival Inventory & Crafting

Press **E** to open the backpack and recipe book. **All recipes** displays missing ingredients and crafting station requirements; **Ready to craft** filters directly to available recipes. Select a recipe, then click **Craft** or **Craft max** (up to 64 batches). Outputs must fit in the backpack; batches that do not fit leave ingredients untouched.

- **Left-click slot**: pick up, place, merge, or swap stacks.
- **Right-click slot**: split half (rounding up) or place a single item.
- **Shift + left-click slot**: rapid transfer between backpack and hotbar.
- **Hover slot**: view item name, stack count, and remaining tool durability.
- **E / ESC / X**: close inventory safely without discarding held cursor items.

Right-click a placed workbench to access 3x3 crafting recipes. Wood-based recipes automatically accept any matching wood/plank variant available in your inventory.

### Item Definitions, Durability & Mining Rules

- `item_registry.go` provides centralized item definitions (stack limits, durability, block placement mappings). `ItemStack` tracks ID, count, and damage state across inventory, cursor, containers, and ground entities.
- Tools stack to 1. Max durability: Wood (59), Stone (131), Iron (250), Diamond (1561), Gold (32). Each block broken in survival consumes 1 durability. Breaking non-effective blocks also costs durability; instant-break blocks and creative mode consume none. Broken tools disappear.
- Mining hardness profiles are centralized in `mining_data.go`. Stone-tier blocks require an appropriate pickaxe to drop resources. Hand-mining yields wood and dirt. Glass, ice, leaves, and tall grass have no ordinary drops (shears and silk touch are not yet implemented).

### Containers: Chests & Furnaces

- **Chests**: Craft a 27-slot chest from 8 matching wooden planks at a workbench. Right-click to open, Shift+Right-click to place against containers. Shift-click rapidly transfers stacks between chest and player inventory.
- **Furnaces**: Craft from 8 cobblestone. Smelts iron/gold ore into ingots, sand into glass, cobblestone into stone, and raw porkchops into cooked porkchops.
  - Smelting speed: 10 seconds per item.
  - Fuel durations: Coal (8 items), Coal Block (80 items), Wood/Planks (1.5 items), Sticks (0.5 items).
  - Furnace burn progress continues in loaded chunks even when the UI is closed.
  - Breaking a chest or furnace drops all stored items and fuel into the world.
- Container state is stored per-position in `containers.json` and synchronized over transaction-validated server sessions.

### Movement, Health & Respawn

- Survival players have 20 Health points and 15 seconds of underwater breath.
- Server applies fall damage (water cushions falls), drowning, lava, lingering fire, suffocation, and void damage.
- Staying out of danger slowly regenerates 1 HP every 10 seconds.
- On death, items held on cursor and in backpack drop at the death site. Clicking **Respawn** teleports the player near their saved spawn anchor with restored health. If ground is unsafe, a small cobblestone landing platform is generated.

### Data-Driven Entities & Mobs

- Mobs use data-driven definitions loaded from `content/`:
  - `content/entities/pig.json`: health, movement speed, collision box, wander/flee durations, and spawn caps.
  - `content/models/pig.json`: parent-relative joint pivots, part dimensions, and 16px/block box UV mappings.
  - `content/animations/quadruped.json`: stride frequencies, leg phases, and head pitch.
- Run `./gocraft.exe -mob-preview` to enter the interactive model workshop:
  - Keys **1–5**: switch animations (Idle, Walk, Flee, Hurt, Death).
  - **A / D**: rotate model view; **B**: toggle collision wireframe; **R**: live-reload JSON files.
- Pigs spawn naturally on surface grass near players (up to 8 pigs per area, 64 global cap). Attacking triggers flee AI; death drops raw porkchops.

### Classic Biomes, River Channels & Fog

- `generation_biomes.go` implements a continuous 6-region climate model:
  - Rolling Plains and mixed Oak/Birch Forests.
  - Arid Deserts with sand dunes, sandstone bedrock, cacti, and dead bushes.
  - Cold Spruce Taiga and Snowy Plains with frozen coastal ice and snow.
  - Extreme Hills / Mountains with ridged peaks, valleys, and slope-driven rock exposure.
- **River Channels**:
  - Deterministic 900-block scale warped noise contours carve valleys down to sea level (Y=61) with riverbeds at Y=57.
  - Gravel-over-dirt riverbeds in temperate regions, sand riverbeds in deserts, and surface ice in cold biomes.
- Atmospheric distance fog smoothly interpolates color and density between biomes.

### World Saves & Atomic Persistence

- All persistence routines (`save_file.go`) utilize atomic write patterns: data is written to a temporary `.tmp` file and committed via atomic `os.Rename`.
- Crash-resilient: sudden power loss or process termination will never leave 0-byte truncated chunk or container files.
- Chunk voxel and lighting data are compressed using high-performance `zstd` blocks.
- Backward compatibility: Entity saves (version 9) and player saves (version 2) automatically migrate legacy formats on load.

---

## Verification & Diagnostics

Run the full automated test suite:
```bash
go test -count=1 ./...
```

Run the standalone generation test harness:
```bash
go run ./tests/generation
```

Export CPU terrain diagnostic maps (hillshaded height, slope, surface coverage):
```powershell
$env:GOCRAFT_REVIEW_DIR = 'work/generation-review'
go test -run TestGenerationReviewMaps -v .
Remove-Item Env:GOCRAFT_REVIEW_DIR
```

Run graphical preview tests (screenshots written to `work/`):
```powershell
# Inventory UI preview
$env:GOCRAFT_INVENTORY_PREVIEW='1'; go test -run '^TestInventoryPreview$' -count=1 .

# Container UI preview
$env:GOCRAFT_CONTAINER_PREVIEW='1'; go test -run '^TestContainerPreview$' -count=1 .

# Mob model rendering preview
$env:GOCRAFT_MOB_PREVIEW='1'; go test -run '^TestMobRenderPreview$' -count=1 .
```

---

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

### Third-Party Licenses

- **Raylib**: Licensed under the zlib License. See [LICENSE_raylib.txt](LICENSE_raylib.txt).
- **mathgl**: Licensed under the BSD-2-Clause License.
- **klauspost/compress**: Licensed under the BSD-3-Clause / Apache 2.0 License.
# Render distance

Settings includes a render-distance slider (8–32 chunks, default 24). Changes
are saved in settings.json and applied to client chunk requests, visibility,
fog and client unloading. The server accepts bounded requests through the
maximum supported range and keeps an additional unloading margin. Its initial
prefetch remains 16 chunks; client requests fill the selected distance.
Higher settings increase mesh memory, loading time and draw cost; 32 chunks
cover roughly four times the area of 16. River generation changes require new
chunks/a new world; existing saved terrain is not regenerated.
# Code navigation

- [Feature-to-file index](CODE_INDEX.md): entry points, subsystem ownership and tests.
- [Generated symbol index](CODE_SYMBOLS.md): types and functions grouped by file.
- Update after structural changes: `go run ./tools/codeindex -write`.
- Check before committing: `go run ./tools/codeindex -check`.

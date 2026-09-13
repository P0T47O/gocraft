# GoCraft

![GoCraft Logo](assets/logo.png)

[English](README.md) | [中文](README_zh.md)

> [!IMPORTANT]
> **Project Status: Early Alpha**. GoCraft is currently a technical demonstration focusing on voxel engine mechanics. Many features are incomplete and significant bugs may exist.

**GoCraft** is a high-performance voxel engine experiment written in Go...

![GoCraft](https://img.shields.io/badge/Language-Go-blue.svg)
![License](https://img.shields.io/badge/License-GPLv3-blue.svg)

## Features

- **Improved UI & UX**:
  - Modern, centered menu layouts for all pages.
  - Professional voxel-style project logo.
  - In-game **Pause Menu** (ESC) with "Save & Quit" functionality.
- **Customizable Settings**:
  - Persistent settings saved to `settings.json`.
  - **Resolution Support**: 16:9, 16:10, 21:9, and 4:3 display modes.
  - **Mouse Sensitivity**: Adjustable slider for refined control.
- **Multiplayer Support**:
  - Authoritative TCP Server/Client architecture.
  - Entity interpolation and synchronization.
  - Graceful server shutdown and resource cleanup.
- **Procedural World**:
  - Infinite terrain generation using Simplex noise.
  - Biome systems with smooth color transitions (Grass, Water).
  - Cave generation and ore veins.
- **Advanced Lighting**:
  - Sky light propagation and day/night cycles.

## 🚧 Current Development Status (WIP)

- **Gameplay Mechanics**: Survival elements, crafting, and mob AI are in early stages.
- **Inventory**: Basic UI is implemented, but advanced item management is ongoing.
- **Audio**: Sound effects and music are yet to be integrated.
- **Optimization**: While fast, large-scale concurrent chunk generation is still being tuned.

## Getting Started

### Prerequisites

- [Go](https://go.dev/dl/) 1.20 or higher.
- A C compiler (GCC/MinGW) is required for cgo (used by raylib).

### Running the Game

1. **Clone the repository:**
   ```bash
   git clone https://github.com/yourusername/gocraft.git
   cd gocraft
   ```

2. **Run the Client (Singleplayer/Default):**
   ```bash
   go run .
   ```

   **Note on Textures**: 
   This repository does not include copyrighted game assets. The game will run with fallback placeholders (checkerboard patterns) by default. To use a resource pack:
   1. Locate or create a `textures/` folder in the game directory.
   2. You can use standard Minecraft resource packs (Java Edition 1.20+ recommended).
   3. Open the resource pack `.zip` file, navigate to `assets/minecraft/textures/`.
   4. Extract the contents (folders like `block`, `item`, etc.) into your local `textures/` folder.
   
   *Disclaimer: You must ensure you have the legal right to use any texture packs or assets you import into the game.*

### Multiplayer

To start a dedicated server:
```bash
go run . -server
```

To join as a specific user (Client):
```bash
go run . -name PlayerName
```

## Controls

- **ESC**: Toggle Pause Menu
- **W, A, S, D**: Move
- **Space**: Jump / Swim Up / Fly Up
- **Left Control + W** or **double-tap W**: Sprint
- **Shift**: Sneak at edges / Swim Down / Fly Down
- **F1**: Toggle Survival / Creative Mode
- **Left Click**: Break Block
- **Right Click**: Place Block
- **1-9**: Select Block from Hotbar
- **E**: Open Inventory
- **F3**: Toggle Debug Info

### Survival inventory and crafting

Press **E** to open the recipe book and backpack. **All recipes** shows missing
materials and workbench requirements; **Ready to craft** filters to recipes you
can make now. Select a recipe, then click **Craft** or **Craft max** (up to 64
batches per click). Shift-clicking Craft also makes a batch. Outputs must fit in
the backpack; a batch that does not fit leaves its ingredients untouched.

- **Left click a slot**: pick up, place, merge, or swap a stack.
- **Right click a slot**: take half (rounding up) or place one item.
- **Shift + left click a slot**: transfer between backpack and hotbar; finish
  placing any item held on the cursor first.
- **Hover a slot**: view its item name and quantity.
- **E / Escape / X**: close the inventory without discarding held items.

Right-click a placed workbench to open workbench recipes. Recipes with equivalent
wood variants share one row, using a variant available in your backpack.

The UI uses existing item icons and code-drawn panels. To check its real rendering
on Windows, run `$env:GOCRAFT_INVENTORY_PREVIEW='1'; go test -run '^TestInventoryPreview$' -count=1 .`
in PowerShell. Captures are written to the ignored `work/` directory. Clear the
environment variable afterward to skip the graphical preview during regular tests.

### Menus and visual theme

The main menu, world browser, settings, pause overlay, inventories and hotbar share
the same dark pixel theme and green selection states. Menu backgrounds are drawn
in code and require no image pack. The world browser scrolls; deleting a world
requires a second click on **Confirm delete**. Press **Escape** to go back from a
menu page. The pause overlay provides **Resume**, **Settings**, and a return to the
main menu; a multiplayer server continues running while its menu is open.

For real-render menu captures, run
`$env:GOCRAFT_MENU_PREVIEW='1'; go test -run '^TestMenuPreview$' -count=1 .`
in PowerShell. This opt-in test uses sample settings/world entries and writes
screenshots to `work/` without changing your saves or settings. Clear the variable
afterward for normal test runs.

### Movement, health and respawn

Survival players have 20 health and 15 seconds of underwater air. The server
applies fall, drowning, lava, lingering fire, suffocation and void damage.
Water cushions falls and extinguishes fire. Without a food system yet, staying
out of danger slowly restores one health about every ten seconds.

Death drops the backpack and cursor items at the death location. Use **Respawn**
to return near your saved spawn anchor with full health and air. Terrain loads
before movement resumes. If no safe ground remains nearby, a small cobblestone
landing is created. Health, air, fire and the anchor are saved; older player
records start with full health. Singleplayer pause stops simulation; multiplayer
menus do not pause the remote server. Update client and server together.

Sprint using **Ctrl + W** or double-tap **W**. **Shift** slows movement and prevents
walking off an edge while grounded; it does not prevent deliberate jumps.
In water, **Space** swims up and **Shift** dives. Food and hunger are not implemented yet; basic mob combat is described below.

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

### Third-Party Licenses

- **Raylib**: Licensed under the zlib License. See [LICENSE_raylib.txt](LICENSE_raylib.txt) for details.

### Mining rules

Mining profiles live in `mining_data.go`; tools use the item registry directly.
Registration rejects physical blocks without an explicit profile. Client timing and
server drops share harvest eligibility, with tool speed separate from harvest tier.
Wood/dirt can be collected by hand; stone materials require a sufficient pickaxe.
Glass, ice, leaves and tall grass have no ordinary drops for now (silk touch,
shears and random plant loot are not implemented). Iron/gold/lapis ores and glowstone
retain their existing self-drops until their resource/processing systems are added.
Mining duration is still client-driven; server-authoritative mining sessions remain
future work for multiplayer. Existing block IDs and saves are unchanged.

### Item definitions and durability

Physical blocks live in `block_registry.go`; `item_registry.go` owns item definitions
(stack limits, maximum durability, tools and block-placement mappings). `ItemStack`
holds ID, count and damage. Inventory, cursor and dropped entities share that state;
`MoveStack`, `CanStack` and `StackLimit` centralize transfer and stacking rules.
The renderer caches standalone item visuals without putting tools in the block registry.

Tools stack to one. Wood/stone/iron/diamond/gold tools have 59/131/250/1561/32 uses.
Each successful survival break of a nonzero-hardness block consumes one use, including
using an ineffective tool. Instant blocks and creative mode consume none. Exhausted
tools disappear; inventory/hotbar bars and hover text show remaining durability.
Q drops the selected instance, preserving its damage. Food and enchantments are not
implemented yet.

Player saves are version 2; legacy version 1 tools become undamaged individual tools.
Overflow remains in `PendingItems` and is delivered as inventory space becomes available,
with a reminder on login. Ground entity saves are independently versioned at 8 and load
legacy versions 6/7, splitting old tool stacks without resetting their despawn age.
World/chunk IDs and formats stay unchanged. Network protocol is now version 4; update
both client and server together. Inventory and dropped-item synchronization include damage.

### Chests and furnaces

Craft a 27-slot chest from 8 matching planks, or a furnace from 8 cobblestone,
at a workbench. Right-click to open; Shift-right-click places against the block.
Left-click moves stacks, right-click splits/places one, and Shift-click transfers
between the container and inventory. Furnace input accepts smeltable items, fuel
accepts supported fuels, and the output slot only permits extraction.

Furnaces smelt iron/gold ore into ingots, sand into glass and cobblestone into stone.
Each item takes 10 seconds. Coal burns for 8 items, a coal block for 80, wood/logs
for 1.5 and sticks for 0.5. Active fuel continues burning if the output is blocked;
new fuel is only ignited when there is room to smelt. Furnaces run while the world
simulation runs, including with the window closed or distant chunks unloaded.
Singleplayer pause and exiting the world stop progress; offline time is not simulated.

`containers.json` stores per-position contents and furnace timers. Existing worlds
need no conversion. Breaking a container drops its contents (including worn tools),
closes viewers and discards remaining furnace heat. Chest blocks currently reuse
existing wooden barrel textures; double chests and opening animations are not added.

Server container sessions validate distance, block type, session token and inventory
revision. Stale simultaneous clicks resynchronize instead of applying twice. Network
protocol 4 requires updating the client and server together. Optional rendered previews:
`$env:GOCRAFT_CONTAINER_PREVIEW='1'; go test -run '^TestContainerPreview$' -count=1 .`

### Data-driven mobs

The placeholder pig logic is replaced by `MobEntity`, data definitions, configurable
feet-origin collision, server AI/combat and a joint-model renderer. Player movement
reuses the collision implementation through an eye-to-feet adapter; player controls,
health and respawn policies remain separate. Mobs use radians and feet-center positions.

Edit `content/entities/pig.json` for health, speed, collider, wander/flee timing,
spawn limit/radius and loot count; `content/models/pig.json` for parent-relative pivots,
part dimensions and box UVs; `content/animations/quadruped.json` for stride and joint
phases. Files in `content/` are loaded on game startup, with embedded defaults when the
directory is absent. Runtime definitions stay fixed for the session. Both multiplayer
peers should use matching content and protocol 4.

Run `./gocraft.exe -mob-preview` for the model workshop: 1 idle, 2 walk, 3 flee,
4 hurt, 5 death; A/D rotate, B collision bounds, R reload files. Reload validates the
replacement before discarding the old preview. Units are blocks, UVs are pixels at
16 pixels per block, and animation angles are radians. Parents must precede children.
No downloaded model or external animation pack is required; the pig reuses the
existing temperate-pig texture. In-game missing visual resources fall back to a
placeholder without disabling simulation.

Pigs wander, stop, climb single-block obstacles, avoid high drops and unloaded terrain,
and flee when hit. Click a nearby pig to attack; the server checks aim, range, walls,
cooldown and tool wear. Death drops 1-3 raw porkchops once, which furnaces cook into
cooked porkchops. Eating/hunger, breeding, advanced pathfinding and full environmental
mob damage remain future work. Grass-surface spawning runs every ten seconds near
connected players, with an eight-pig default cap (64 total mobs maximum).

Entity save version 9 preserves mob health, motion, RNG, behavior and death/drop state.
Old pig records in versions 6-8 migrate to the new origin/angle convention. Inventory,
container and chunk formats remain compatible. Automated tests cover shared collision,
steps, cliffs, attack obstruction/cooldown, single death loot, resource validation and
save migration. Optional visual test: `GOCRAFT_MOB_PREVIEW=1 go test -run TestMobRenderPreview`.

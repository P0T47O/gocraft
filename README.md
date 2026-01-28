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
- **Space**: Jump / Fly Up
- **Left Control**: Fly Down
- **F**: Toggle Flying Mode
- **Left Click**: Break Block
- **Right Click**: Place Block
- **1-9**: Select Block from Hotbar
- **E**: Open Inventory
- **F3**: Toggle Debug Info

## License

This project is licensed under the GNU General Public License v3.0 - see the [LICENSE](LICENSE) file for details.

### Third-Party Licenses

- **Raylib**: Licensed under the zlib License. See [LICENSE_raylib.txt](LICENSE_raylib.txt) for details.

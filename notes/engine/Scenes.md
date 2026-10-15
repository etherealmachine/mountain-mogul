---
title: Scenes
kind: engine
status: shipped
---

# Scenes

The app runs a stack of scenes, each with init, update, render, and destroy. Launch shows a studio splash, then the start menu. From there:

- New Game opens the scenario picker, which lists the bundled scenarios in `assets/scenarios/` (`boreal.save` and `kirkwood.save`) and starts gameplay from the one picked.
- Games lists the player's games, to continue, rename, copy or delete ([[Saved Games]]); a scenario with a game in progress offers Continue or New game in the picker.
- The [[Scenario Editor]] opens a scenario, or a blank world, with no simulation running.
- [[Terrain Import]] builds a new map from real-world elevation.
- The testbed menu lists every [[Testbeds]] world.

The gameplay scene holds the simulation, the renderer, every build tool (lifts, buildings, roads, terrain brushes, grooming routes, parking lots, [[Lodge Shell]] tiles, [[Trails]]), the [[UI and HUD]], and the [[Debug Tools]]. Escape opens a menu with save, rename, load, and settings; the game also saves itself on starting, each midnight, and on leaving.

Code: `internal/engine/`, `internal/scene/`.

## Log

- 2026-10-01: Splash, start menu, scenario picker, gameplay, editor, import, save list, and testbed menu are in.
- 2026-10-15: Saved games: the games list, Continue or New game in the picker, Rename in the escape menu, autosave ([[Saved Games]]).

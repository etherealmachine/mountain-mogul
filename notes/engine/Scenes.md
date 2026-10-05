---
title: Scenes
kind: engine
status: shipped
---

# Scenes

The app runs a stack of scenes, each with init, update, render, and destroy. Launch shows a studio splash, then the start menu. From there:

- New Game opens the scenario picker, which lists the bundled scenarios in `assets/scenarios/` (`boreal.save` and `kirkwood.save`) and starts gameplay from the one picked.
- Load opens the list of player saves; see [[Save Format]].
- The [[Scenario Editor]] opens a scenario, or a blank world, with no simulation running.
- [[Terrain Import]] builds a new map from real-world elevation.
- The testbed menu lists every [[Testbeds]] world.

The gameplay scene holds the simulation, the renderer, every build tool (lifts, buildings, roads, terrain brushes, grooming routes, parking lots, [[Lodge Shell]] tiles, [[Trails]]), the [[UI and HUD]], and the [[Debug Tools]]. Escape opens a menu with save, load, and settings.

Code: `internal/engine/`, `internal/scene/`.

## Log

- 2026-10-01: Splash, start menu, scenario picker, gameplay, editor, import, save list, and testbed menu are in.

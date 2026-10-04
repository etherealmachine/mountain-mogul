---
title: Terrain
kind: system
status: shipped
---

# Terrain

A height-map grid of 5 m cells, imported from real-world elevation tiles or edited by hand. Each cell carries ground height, a [[Snow]] pack, its stored [[Trees]], and whether it can be walked on. Editor brushes raise, lower, clear (glade), and plant trees.

Placing a lift station or a painted pad grades the ground beneath it into a level pad. A ramped embankment around it keeps guests from skiing off a wall. Trees matter to [[Skiing]] (guests avoid trees, and glade lovers seek them), to [[Avalanche]] (trees anchor slopes), and to how deep snow buries a trunk.

[[Parcels]] decide which parts of the terrain the player may build on.

Spec: the elevation contract and apron pass in [[Snow Spec]]. Code: `internal/world/terrain.go`, `forest.go`, `internal/geo/`, `internal/scene/embankment.go`.

Cliffs, creeks, and a less uniform look are planned in [[Terrain Realism]].

## Log

- 2026-10-01: Real-world import, brushes, trees, aprons, and embankments are in.
- 2026-10-02: Trees are stored individually instead of as a density per cell.

---
title: Pathfinding
kind: system
status: partial
---

# Pathfinding

How guests get around on foot. An A* search over the terrain grid routes walkers between parking, building doors, and lift bases. Building cells and lift ends block the way, and a walking guest draws with skis off. [[GOAP]] steps like walking to a lift or a ticket window start a route; when no route exists, the guest gives up on that step, or for a new arrival, is never spawned.

[[Skiing]] does not use this. Runs are steered, not routed.

Not built yet: painted footpaths between buildings, ski racks where paths meet the snow (see [[Parking and Roads]]), and lift lines routed around buildings.

Code: `internal/sim/pathfinder.go`.

## Log

- 2026-10-01: Grid A* walking to doors, lift bases, and parking is in. Footpaths are not.

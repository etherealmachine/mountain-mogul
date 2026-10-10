---
title: Pathfinding
kind: system
status: partial
---

# Pathfinding

How guests get around on foot. An A* search over the terrain grid routes walkers between parking, building doors, and lift bases, in eight directions (a diagonal costs √2 and never cuts a blocked cell's corner), and the route is cut down to its corners: from each kept cell, the furthest one a straight walk to stays on open cells (checked a metre either side, for the walker's shoulders) and on a footpath when both ends are on one. Walkers go straight from corner to corner. Building cells, lift ends, cells with two or more tree trunks, and land the resort can't use block the way; the destination may be blocked (a lift base, a door in a wall). A walking guest draws with skis off. Patrollers on foot use the same search. It keeps its working arrays between searches (the sim calls it serially). [[GOAP]] steps like walking to a lift or a ticket window start a route; when no route exists, the guest gives up on that step, or for a new arrival, is never spawned.

[[Skiing]] does not use this. Runs are steered, not routed.

**Footpaths** (`world/footpath.go`): the player lays them with the Path tool under Transport (game and editor; `scene/footpath_tool.go`), node by node like a run, 1.5–8 m wide (3 by default; [ and ] change the next node's width), clicking the last node again or Enter to finish, right-click to take back a node; nodes snap to existing paths so they join. They cost $10 a square metre, need owned land in the game, and the Remove tool takes one out. Guests on foot walk twice as fast on a path (`world.FootpathSpeedup`, `sim.walkSpeed`). Both walking searches prefer them: the grid A* counts a path cell as half a step, and the route planner for walkers with skis off halves a path cell's cost, estimates as if the whole way were path (so it finds the detour by a path when it's quicker), always plans when there are paths rather than walking straight, keeps a route between two path cells on the path, and moves route points in a path's cells onto its centre line. Paths are drawn as a strip of gritted snow just under the roads (`render/footpath_mesh.go`) and saved (`footpaths`).

Laid paths are edited with the same tool when not laying one: the path under the cursor shows its node handles and width handles; drag a node to move it (a click that doesn't move it starts a new path from it), drag a width handle or use [ and ] over a node to set the width there, right-click a node to delete it (a path left with one node goes), Shift-click a path to put a node in it. In the game an edit that makes a path bigger charges the extra area, and is undone if that can't be paid. Paths are shovelled like roads are plowed (`applyFootpathCellState` in `scene/road_cells.go`, part of the road pass): snow off every cell whose centre is on the path, fading back over 3 m, and trees within a metre of its edge cut, when a path is laid or edited and at every day rollover.

Not built yet: ski racks where paths meet the snow (see [[Parking and Roads]]), lift lines routed around buildings, and paths that avoid building footprints.

Code: `internal/sim/pathfinder.go`.

## Log

- 2026-10-01: Grid A* walking to doors, lift bases, and parking is in. Footpaths are not.
- 2026-10-02: Trees block a cell when it holds two or more trunks, replacing the old density cutoff ([[Trees]]).
- 2026-10-10: Footpaths: laid with the Path tool, twice as fast to walk, preferred by both walking searches. On Boreal with two test paths from the lots toward the main lift, a morning's walkers spent 13.8% of their walking on paths and averaged 0.78 m/s against 0.68 without.
- 2026-10-10: Footpath editing (drag nodes and widths, insert, delete) and snow clearing (with the roads, overnight).
- 2026-10-10: Walking routes in eight directions, cut to their corners, on arrays instead of maps. On Boreal over a morning, routes went from 26% longer than the straight line with 5.2 turns per 100 m (a staircase along cell centres) to 2% longer with 0.8. No search failed before or after. Visitors on the Goals Test differ day to day (373/452/402 against 421/541/434) from the random stream diverging once lifts open: identical until 9:00, cars 158 against 170, ratings alike.

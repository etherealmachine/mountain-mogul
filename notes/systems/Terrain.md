---
title: Terrain
kind: system
status: shipped
---

# Terrain

A height-map grid of 5 m cells, imported from real-world elevation tiles or edited by hand. Each cell carries ground height, a [[Snow]] pack, its stored [[Trees]], and whether it can be walked on. Editor brushes raise, lower, clear (glade), and plant trees.

Placing a lift station grades a flat apron in front of the lift post instead of building it up. In front is the side skiers face: where they queue and load at the bottom, and ski off after unloading at the top, under the beam and bullwheel, away from the cable. The apron is about 8 m deep and 12 m wide, levelled to the natural ground at its middle, with banks easing back to natural ground over about 8 m at the front and sides. Nothing on the cable side of the post changes. It's flattened in the lidar detail too, packed and groomed, and rock under it becomes ground (`carveStationApron`, `internal/scene/lift_apron.go`). A painted pad is graded into a level pad, with a ramped embankment around it so guests don't ski off a wall. Trees matter to [[Skiing]] (guests avoid trees, and glade lovers seek them), to [[Avalanche]] (trees anchor slopes), and to how deep snow buries a trunk.

[[Parcels]] decide which parts of the terrain the player may build on.

Spec: the elevation contract and apron pass in [[Snow Spec]]. Code: `internal/world/terrain.go`, `forest.go`, `internal/geo/`, `internal/scene/embankment.go`.

Cliffs, creeks, and a less uniform look are planned in [[Terrain Realism]].

## Log

- 2026-10-01: Real-world import, brushes, trees, aprons, and embankments are in.
- 2026-10-02: Trees are stored individually instead of as a density per cell.
- 2026-10-06: Imported terrain gets a material map on the 1.25 m lattice (meadow, dirt, scree, rock kinds) from the Auto material layer; trees don't grow on rock or scree ([[Ground Materials]]).
- 2026-10-06: Lift stations grade a small flat apron in front of the post (the bullwheel side, where skiers queue and ski off) instead of raising a 40 × 24 m pad 2.5 m above the ground (bottom) or shelving one (top); the cable side of the post is left alone.

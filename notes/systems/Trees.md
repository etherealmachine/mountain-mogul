---
title: Trees
kind: system
status: partial
---

# Trees

Forest is a per-cell `TreeDensity` from 0 to 1, saved with the [[Terrain]]. The individual trees are derived from it every time they are needed; they are never stored. [[Stored Trees]] is the plan to change that.

## How density becomes trees

A cell holds at most 2 trees (`MaxTreesPerCell`, kept low for GPU instance cost). Density × 2 is the expected count. The whole part is always drawn, and the fractional part is decided by a fixed per-cell hash: density 0.3 gives one tree in cells whose hash falls under 0.6 and none elsewhere. Each tree's offset within ±1.2 m of the cell centre, its rotation, scale, and model variant also come from a hash of the cell and the tree's index.

None of this is random. The same density always yields the same trees, so the forest is identical across saves, loads, and machines. `ForEachTree` in `internal/world/forest.go` is the one place that turns density into trees. [[Rendering]] uses it for the static batch, and the snow detail texture uses it for tree wells.

Hand-placed single trees are separate: decorative objects in `World.Objects`, drawn alongside the forest.

## What reads density, not trees

Gameplay never sees individual trees, only the smooth density:

- [[Skiing]]: steering treats density as a hazard to route around, balance drains in cells above 0.3, and glade-loving or tree-shy guests react to it ([[Satisfaction]])
- [[Pathfinding]]: a cell at 0.5 or more cannot be walked
- [[Avalanche]]: density anchors a slope against release, and a front stops in cells above 0.7

So a cell at density 0.2 can show no trees and still push skiers away, and a cell showing one tree can be anywhere from about 0.01 to 0.99.

## What writes density

- the glade tool in play (see below)
- the editor's plant brush, which raises density up to a slider cap
- the editor's automatic forest generator ([[Scenario Editor]])
- lift corridors, station aprons, roads, parking lots, and [[Lodge Shell]] footprints, which zero it

## Problem: the glade tool can't say what it will remove

Each glade click lowers density by the thinning slider (0–10%) in every cell under the brush, and charges $200 × the density actually removed. A tree only disappears when a cell's density crosses its hash threshold. Most clicks cost money and change nothing visible, and then a tree pops out somewhere under the brush. There is no preview because the tool doesn't think in trees.

The tutorial map shows the scale: 101,545 forested cells but only 62,804 trees, and all but about 500 of those cells sit at a fractional density, straight out of the forest generator.

## Log

- 2026-10-01: Documented the density model and the glade-tool problem. The fix is planned in [[Stored Trees]].

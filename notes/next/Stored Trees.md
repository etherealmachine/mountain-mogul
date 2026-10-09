---
title: Stored Trees
kind: plan
status: shipped
---

# Stored Trees

Replace per-cell density with individually stored [[Trees]], so the glade tool can show and price exactly what it removes, and gameplay reacts to the trees the player sees. Listed in [[Next Steps]].

## Decision: store every tree, and have gameplay read trunks

Trees become the saved source of truth: each tree is a stored point with its own position. Generators and brushes still think in density, but they place trees, and density stops being saved. Skiing, walking, and avalanches read the trees themselves.

Why explicit positions: trees can sit anywhere, more than two can share a cell, and each can later carry its own state (species, size, felled by an avalanche, hit by a skier). Measured on the tutorial map (379 × 379 cells, 62,804 trees; the whole save is 1.49 MB gzipped):

| Stored as | Gzipped size |
|---|---|
| Density, float per cell (today) | 417 KB |
| Tree positions as floats | 402 KB |
| Tree positions at 10 cm | 222 KB |
| Which hash-derived tree slots exist, one byte per cell | 18 KB |

So explicit floats cost about what density costs now. The slot byte was rejected for now: much smaller, but trees stay pinned to two slots per cell. Quantizing positions to 10 cm is the easy compression if the save grows.

Rejected: keeping density and making the glade brush preview threshold crossings. It needs no save change, but gameplay would keep reading a density the player can't see.

## Steps

1. **Data.** A tree is a world XZ position. Rotation, scale, and model variant derive from a hash of the position, so they aren't stored and look the same after reload. Trees are kept in per-cell buckets on the [[Terrain]], so brushes and skier queries touch only nearby cells. Lone decorative trees in `World.Objects` fold into the same store.
2. **Saves.** Trees are saved as a list of positions, and `TreeDensity` is no longer written. An old save converts on load by running today's density-to-trees rule once, so its forest looks identical. See [[Save Format]].
3. **Writers place trees.**
   - The forest generator keeps its density field, then samples trees from it with a minimum spacing. The two-per-cell cap becomes a rendering budget, not a storage rule.
   - The plant brush adds spaced trees up to a target density.
   - Lift corridors, station aprons, roads, parking lots, and [[Lodge Shell]] footprints remove the trees inside them.
4. **Glade tool.** The brush selects whole trees within its radius and highlights them before the click. The cost label prices exactly those trees, and clicking removes them. The thinning slider becomes the share of trees under the brush to take, picked deterministically so the preview matches the result.
5. **Readers use trunks.**
   - [[Skiing]] scores steering by nearby trunks from the bucket grid, the way it already handles lift towers and other skiers. That makes a collision with one specific tree possible: a fall, a thought, maybe an injury.
   - [[Pathfinding]] blocks a cell when its trunks leave no gap to walk through, instead of the density-0.5 cutoff.
   - [[Avalanche]] counts trunks per cell for slope anchoring and for stopping a front.
   - Glade-loving and tree-shy guests react to trunks nearby ([[Moments]]).
6. **Rendering and snow.** The static batch and tree wells read the stored trees instead of `ForEachTree`.

Steps 1, 2, and 6 together should look exactly like today, which makes them a safe first change to check the conversion before anything else moves.

## Open questions (resolved)

- **Trunk radius.** Steering feels trunks within 3 m, falling off with distance. A hit needs a trunk within 0.6 m while the skier heads at it faster than 2 m/s. Near misses aren't counted yet.
- **Raw trunks or a summary.** Skiing reads raw trunks each tick, but skips the lookup when the cell is already full cover. A micro-benchmark put the full lookup at about 26 ns per call, against 2.7 ns for the cell read alone.
- **Testbeds.** They still stamp a density, through the same conversion old saves use. The tree-patch and curving-trail testbeds give the same outcomes as before.
- **Renderer budget.** Unchanged. Generators still place about two trees per full-cover cell, so the tutorial map has 63,154 trees, about the same as before.

## Not done

- Glade-loving and tree-shy guests still react to the cell's cover, not to trunks nearby.
- The editor's glade brush has no highlight and a fixed 40% share.

## Log

- 2026-10-01: Chose explicit tree positions with gameplay reading trunks. Nothing implemented yet.
- 2026-10-02: Shipped all six steps; see [[Trees]] for how it works now. Checked that converted saves keep the same trunk positions (tutorial screenshots before and after), that the glade preview matches what a click removes (tests and a screenshot), that saves round-trip, and that testbed outcomes are unchanged.
- 2026-10-02: `tutorial.save` re-saved in the stored-tree format (1.44 MB, against 1.49 MB) while adding its scenario name.

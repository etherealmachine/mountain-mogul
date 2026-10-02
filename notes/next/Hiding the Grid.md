---
title: Hiding the Grid
kind: plan
status: planned
---

# Hiding the Grid

Gameplay runs on 5 m cells, and several things draw those cells as they are, so the grid shows through as staircases on anything diagonal or curved. The aim is to keep the cells for gameplay and draw smooth shapes from them. Listed in [[Next Steps]].

## Where the grid shows today

- **Parcel fence** ([[Parcels]]). `buildParcelFence` in `internal/scene/scenario.go` emits one span per owned-cell edge: a post on each cell corner, a rope every 5 m. A diagonal boundary becomes a staircase of right angles. Posts get a small per-corner nudge, which roughens the steps without removing them.
- **Painted overlays** ([[Trails]], parking-lot paint, the land-for-sale highlight, the [[Lodge Shell]] floor plan). `buildCellOverlay` writes one texel per cell; the terrain shader samples it with linear filtering. Edges are blurred over a cell instead of following a shape, and diagonal trails read as soft stairs.
- **Groomed snow** ([[Grooming]]). Grooming is a per-cell value, and a cat grooms the whole cell it stands in. The terrain shader averages it to cell corners, and the crisp corduroy-to-powder edge comes from a 1 m band laid along every cell boundary where grooming steps (`RecomputeGroomEdges`). A groomed run that curves shows square corners and straight edges.

## Approach

Draw from the cells, never as them. Cell data stays the source of truth for gameplay, saves, and tools; each renderer turns it into a smooth shape first.

1. **Fence follows a smoothed outline.** Trace each owned region's boundary into a closed loop, cut the corners off staircases (a couple of rounds of corner cutting turns 5 m steps into a smooth diagonal), then place posts at an even spacing along the curve with a little jitter. Ropes sag between those posts as now.
2. **Overlays get clean, organic edges.** Keep the per-cell texture, but in the shader treat its alpha as a mask: nudge the sample position with low-frequency noise (a metre or two) and sharpen with a smoothstep. Diagonals come out as smooth lines instead of steps, edges look hand-drawn instead of blurred, and a thin darker outline can trace each trail. One change in `terrain.frag` covers trails, parking paint, land highlights, and floor plans.
3. **Grooming follows the cat.** Keep per-cell grooming for gameplay (friction, satisfaction, the nightly "needs a pass" check), but draw corduroy from where the cat actually drove. As a cat moves, it stamps its 5 m swath along its real path into a new channel of the 1 m surface detail texture. The stripes and the groomed edge read from that channel, so lanes curve with the route and meet at angles the cat actually turned. Skier wear fades the stamp the same way it lowers cell grooming.

Order: the overlay shader change is the smallest and fixes the most surfaces at once. The fence is self-contained. Grooming is the biggest, because it touches the snowcats, the detail texture, and the terrain shader.

## Open questions

- Should the noise that roughens overlay edges be fixed per world, so a trail's outline doesn't change while you paint next to it?
- How much smoothing can the fence take before it visibly cuts across cells the player owns, or leaves out ones they do?
- Does the groomed-path stamp need saving, or is regrooming on load from cell values good enough? The detail texture is not saved today (see [[Snow]]).
- Should trail painting itself move off the grid later (a brushed shape that gets rasterised to cells), or is drawing the cells smoothly enough?

## Log

- 2026-10-01: Documented where the grid shows (fence, overlays, grooming) and planned smoothing each from cell data. Nothing implemented yet.

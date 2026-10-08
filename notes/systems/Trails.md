---
title: Trails
kind: system
status: partial
---

# Trails

A run is drawn as a line of nodes, each with its own width, usually from a lift top to a lift base; its ends attach to lift stations, buildings, or other trails and follow them ([[Trail Network]]). The cells it covers are derived from that shape, and the game builds a graph from what each trail's cells touch: lift tops and bases, buildings, parking, and other trails. Glades, bowls, and backcountry will be outlined areas (planned).

[[GOAP]] plans runs along that graph. Guests don't follow a trail's cells once on it: they ski toward the run's destination, detouring round trees ([[Skiing]]); leaving the trails costs nothing extra yet. A lift serves the difficulties of the trails off its top, which is how [[Demand]] decides whether there is terrain for a guest's skill. Trails draw as smooth ribbons in their difficulty colour with their names.

Not built yet, from [[Vision]]: trail closures and slow zones.

Spec: [[Trails Spec]].

## Log

- 2026-10-01: Painted trails, the derived graph, trail-aware planning, and difficulty-based lift service are in. Closures and slow zones are not.
- 2026-10-07: Each trail keeps `Conditions`, the average of its cells' snow features, refreshed every clock hour; a lift's conditions (the trails off its top) feed guests' lift choice ([[Snow Tastes]]).
- 2026-10-08: Corrected: leaving the trails isn't penalised in the code. Node-based trails and zones planned ([[Trail Network]]).
- 2026-10-08: Trails are drawn as nodes with widths instead of painted; ends attach to lifts; painted trails no longer load ([[Trail Network]]).

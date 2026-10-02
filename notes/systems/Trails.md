---
title: Trails
kind: system
status: partial
---

# Trails

A trail is a painted area of cells with a difficulty (green, blue, or black), not a drawn route. The game builds a graph from what each trail touches: lift tops and bases, buildings, parking, and other trails. Painting a trail is all it takes to connect them.

[[GOAP]] plans runs along that graph, and leaving the trails costs beginners much more than experts. A lift serves the difficulties of the trails off its top, which is how [[Demand]] decides whether there is terrain for a guest's skill. Trail names and colors draw on the terrain, one color per cell, so diagonal trails look stepped ([[Hiding the Grid]]). [[Skiing]] decides the line within the painted area.

Not built yet, from [[Vision]]: trail closures and slow zones.

Spec: [[Trails Spec]].

## Log

- 2026-10-01: Painted trails, the derived graph, trail-aware planning, and difficulty-based lift service are in. Closures and slow zones are not.

---
title: Parking and Roads
kind: system
status: partial
---

# Parking and Roads

Parking lots are painted cells with stalls and aisles laid out automatically. The road graph connects map-edge entry points to lot driveways, and crossings become intersections automatically. Every arriving guest starts and ends the day at a lot, and the lot shows one car per four guests. Parking can charge a per-car fee, which feeds [[Demand]] and [[Finance]].

The parking lot is also where [[Ski Patrol]] takes injured guests, and where a ticket window's door faces (see [[Tickets]]).

Not built yet, from [[Vision]] and [[Next Steps]]: pedestrian paths between buildings, guests taking skis off to walk, ski racks, and a parking choice weighted by distance to the lifts.

Code: `internal/world/parking.go`, `road.go`.

## Log

- 2026-10-01: Painted lots, the road graph, cars, and the parking fee are in. Footpaths and ski racks are not.
- 2026-10-06: Planned [[Transit]]: cars driving in from the map's edge with simple traffic, rectangular extendable lots in asphalt, gravel, or dirt, and editor-set entry points.

---
title: Parking and Roads
kind: system
status: partial
---

# Parking and Roads

Parking lots are rectangles, drawn and resized by dragging, with stall rows along the long side, an entrance facing the nearest road and a driveway that meets it at a T. The road graph connects map-edge entry points to lot driveways, and crossings become intersections automatically. Guests arrive in cars of one to four that drive from their entry along the roads, with simple traffic, to a stall in the nearest lot with room, and drive home when the carload is back (see [[Transit]]). Parking can charge a per-car fee, which feeds [[Demand]] and [[Finance]].

The parking lot is also where [[Ski Patrol]] takes injured guests, and where a ticket window's door faces (see [[Tickets]]).

Not built yet, from [[Vision]] and [[Next Steps]]: pedestrian paths between buildings, guests taking skis off to walk, ski racks, and a parking choice weighted by distance to the lifts.

Code: `internal/world/parking.go`, `road.go`, `car.go`, `internal/sim/traffic.go`.

## Log

- 2026-10-01: Painted lots, the road graph, cars, and the parking fee are in. Footpaths and ski racks are not.
- 2026-10-06: Planned [[Transit]]: cars driving in from the map's edge with simple traffic, rectangular extendable lots in asphalt, gravel, or dirt, and editor-set entry points.
- 2026-10-06: Map-edge entries have a name and a guest pool, set in the editor; every guest lives beyond one entry and arrives and leaves by it ([[Transit]]).
- 2026-10-06: Cars drive: carloads of one to four from their entry to a stall and back, with lanes, following, and junctions taken in turns ([[Transit]] step 2). Road dead ends touching a lot count as its entrances.
- 2026-10-06: Lots are rectangles instead of painted cells ([[Transit]] step 3). Embankments no longer cut into roads, and roads are drawn on the terrain mesh's own height.

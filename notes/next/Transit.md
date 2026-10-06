---
title: Transit
kind: plan
status: planned
---

# Transit

How guests get to the mountain: cars entering at the map's edge, driving the roads, and parking in lots, with simple simulated traffic from the start, since getting people in and out is a key part of running a resort. Rectangular lots replace painted ones, in asphalt, gravel, or dirt. Builds on [[Parking and Roads]]; listed in [[Next Steps]].

## Today

- Lots are painted 5 m cells: a staircase outline, stall rows at one angle with no clear aisles, and a plowed fringe. Asphalt only.
- Driveways don't join cleanly: at [[Boreal]] one dead-ends at a corner of the lot, another meets the lot edge at an angle with a gap, and there's a stub by the junction.
- No car drives. A lot keeps a car count (one car per four guests), cars appear in stalls, and guests appear at the lot. The road from the map edge is drawn but carries nothing.
- Edge connections already exist as a road node kind the editor places (`RoadNodeEdgeConnection`), but nothing uses them.

## Decisions

Made with the user on 2026-10-06:

- **Simulated traffic up front**, kept simple at first and improved later. Cars are agents on the road network; arrival takes the drive's time, roads can back up, and a full lot turns cars away.
- **Rectangular lots for now**: dragged and rotated, extendable by dragging an edge, for a cost. Polygon lots wait for [[Gridless Drawing]].
- **Surfaces: asphalt, gravel, dirt.** They differ in cost, look, and capacity. No spring-mud effects. Later, cars drive more slowly on gravel and dirt.
- **The scenario editor controls the entry and exit points**, which can feed the demand model (where guests come from). The import doesn't suggest them.
- **Carloads of one to four guests** for now; bigger vehicles (vans, buses) can come later.
- **Speeds:** the one road type, a two-lane road, at 35 mph (about 56 km/h); cars in lots and on driveways at 5–15 mph (about 8–24 km/h), slowest in the aisles.

## Steps

1. **Entry and exit points.** Edge connections become proper entries in the editor: placed where a road meets the map's edge, named ("I-80 west"), and each with a share of arrivals. The demand model sends each arriving car through an entry by share; departing cars leave by an exit (the nearest, or the one they came in by).
2. **Cars on the road.** A car spawns at its entry when its guests decide to come, drives the shortest route to a lot at the road's speed, parks in a stall, and its guests (up to four) get out. In the evening they walk back, get in, and drive to an exit. Simple traffic: cars keep a gap to the car ahead and don't overtake; at intersections the first to arrive goes; a lot's entrance takes one car at a time. A full lot sends a car on to the next lot, or home. Cars are saved.
3. **Rectangular lots.** A drag-and-rotate rectangle tool. Stall rows and aisles run along the long side; rounded corners; one entrance on the side facing the nearest road, with a driveway that meets the road at a proper T and a gap in the plowed fringe. Drag an edge to extend, paying for the added area. Painted lots are replaced (breaking old saves is fine pre-release); Boreal's lot is redrawn as a rectangle.
4. **Surfaces.** Asphalt, gravel, or dirt per lot: cost per square metre, capacity (gravel and dirt have no striping, so cars pack less neatly and fewer fit), and look (asphalt with lines, light grey gravel, packed dirt and snow).
5. **Later.** Slower driving on gravel and dirt; better traffic (merging, turning lanes, signals); road closures (*for* [[Alta]]); trains (*for* [[Zermatt]]); parking choice weighted by distance to the lifts.

## Open questions

- How many cars at once before performance matters: a busy day at a big resort is thousands of cars arriving over a morning.

## Log

- 2026-10-06: Planned with the user after looking at Boreal's lot and roads: simulated traffic up front, rectangular extendable lots, asphalt, gravel, and dirt, and editor-controlled entry and exit points tied to demand.
- 2026-10-06: Decided: carloads of one to four; 35 mph on the two-lane road, 5–15 mph in lots; no import suggestions for entry points.

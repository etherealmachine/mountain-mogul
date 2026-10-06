---
title: Transit
kind: plan
status: in progress
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

1. Done: **Entry and exit points, with guest pools.** The editor's Edge tool (under Transport) places a post on the map's edge with a short road stub, named "Entry N", with a guest pool of 5,000. Clicking a post (with the Edge tool or no tool) opens a Road entry popup: the name, the guest pool (steps of 500, up to 1,000,000), its share of all guests, and Delete (which also clears the stub's orphan node). Entries are `RoadNodeEdgeConnection` nodes with `Name` and `Pool` (saved as `name`, `pool`). Every guest in the pool lives beyond one entry (`Guest.HomeEntryID`, saved as `entry`), so they always arrive and leave by it, and an entry's share of arrivals follows from its pool. `world.SyncGuestPool` makes the pool match the entries on every load: guests with no matching entry fill entries that are short before new ones are rolled, and surplus guests at home are dropped. Without entries a map keeps the default 10,000-guest pool. Checked on Boreal: entries of 30,000 and 10,000 give 40,000 guests split exactly (Boreal's existing 10,000 reused), changing to 5,000 and 20,000 follows, and deleting one hands its guests on; loading takes about 120 ms. Boreal has no entries yet: its road reaches the map's edge, but no post is placed.
2. **Cars on the road.** A car spawns at its entry when its guests decide to come, drives the shortest route to a lot at the road's speed, parks in a stall, and its guests (up to four) get out. In the evening they walk back, get in, and drive to an exit. Simple traffic: cars keep a gap to the car ahead and don't overtake; at intersections the first to arrive goes; a lot's entrance takes one car at a time. A full lot sends a car on to the next lot, or home. Cars are saved.
3. **Rectangular lots.** A drag-and-rotate rectangle tool. Stall rows and aisles run along the long side; rounded corners; one entrance on the side facing the nearest road, with a driveway that meets the road at a proper T and a gap in the plowed fringe. Drag an edge to extend, paying for the added area. Painted lots are replaced (breaking old saves is fine pre-release); Boreal's lot is redrawn as a rectangle.
4. **Surfaces.** Asphalt, gravel, or dirt per lot: cost per square metre, capacity (gravel and dirt have no striping, so cars pack less neatly and fewer fit), and look (asphalt with lines, light grey gravel, packed dirt and snow).
5. **Later.** Slower driving on gravel and dirt; better traffic (merging, turning lanes, signals); road closures (*for* [[Alta]]); trains (*for* [[Zermatt]]); parking choice weighted by distance to the lifts.

## Open questions

- How many cars at once before performance matters. The goal is thousands: a busy day at a big resort is thousands of cars arriving over a morning.

## Log

- 2026-10-06: Planned with the user after looking at Boreal's lot and roads: simulated traffic up front, rectangular extendable lots, asphalt, gravel, and dirt, and editor-controlled entry and exit points tied to demand.
- 2026-10-06: Decided: carloads of one to four; 35 mph on the two-lane road, 5–15 mph in lots; no import suggestions for entry points.
- 2026-10-06: Step 1: named entries, each with its own guest pool (the user's call, replacing a share of arrivals); every guest belongs to one entry and arrives and leaves by it. The goal for step 2 is thousands of cars.

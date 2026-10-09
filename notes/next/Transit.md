---
title: Transit
kind: plan
status: partial
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
- **Carloads of one to four guests** at first; vans of up to seven came with [[Groups]]. Buses and RVs, which don't fit a stall, are in [[Large Vehicles]].
- **Speeds:** the one road type, a two-lane road, at 35 mph (about 56 km/h); cars in lots and on driveways at 5–15 mph (about 8–24 km/h), slowest in the aisles.

## Steps

1. Done: **Entry and exit points, with guest pools.** The editor's Edge tool (under Transport) places a post on the map's edge with a short road stub, named "Entry N", with a guest pool of 5,000. Clicking a post (with the Edge tool or no tool) opens a Road entry popup: the name, the guest pool (steps of 500, up to 1,000,000), its share of all guests, and Delete (which also clears the stub's orphan node). Entries are `RoadNodeEdgeConnection` nodes with `Name` and `Pool` (saved as `name`, `pool`). Every guest in the pool lives beyond one entry (`Guest.HomeEntryID`, saved as `entry`), so they always arrive and leave by it, and an entry's share of arrivals follows from its pool. `world.SyncGuestPool` makes the pool match the entries on every load: guests with no matching entry fill entries that are short before new ones are rolled, and surplus guests at home are dropped. Without entries a map keeps the default 10,000-guest pool. Checked on Boreal: entries of 30,000 and 10,000 give 40,000 guests split exactly (Boreal's existing 10,000 reused), changing to 5,000 and 20,000 follows, and deleting one hands its guests on; loading takes about 120 ms. Boreal has two entries where its road meets the map's edge: "I-80 west (Sacramento, Bay Area)" on the west edge with 80,000 guests and "I-80 east (Truckee, Reno)" on the north edge with 20,000 (100,000 in all, sized toward thousands a day; the save grew from 4.9 to 6.9 MB).
2. Done: **Cars on the road.** Each demand poll's winners from one entry share cars of one to four (`world.Car`, saved); a car waits at its entry for room, drives the shortest route to the nearest lot with a stall to spare (counting cars already on their way), claims a stall as it nears the lot, drives in, parks, and its guests get out and pay their shares of the per-car parking fee. Guests leave from their car's lot; each waits in the car (`InCar`), and when the whole carload is aboard the car drives home by the same entry. A car that finds no stall goes on to the next lot with room, or turns around and drives home ("Cars turned away: every parking lot is full", once a day). Traffic is described under How it works. Checked headless on Boreal: about 4,000 cars at once take about 0.4 ms a tick; with arrivals only, the lot takes about one car a second through its two entrances, fills its 505 stalls in about 10 minutes, and turns the rest home.
3. Done: **Rectangular lots.** The Parking tool (game and editor) drags out a rectangle, turned to line up with the nearest road within 120 m (R turns it further); dragging near an edge of a lot (or using Resize in its popup) moves that edge. The game charges $100 per m² added (nothing back for shrinking) plus the driveway, needs owned land, and shows size, stalls and cost while dragging. Stall rows run along the long side with a cross aisle at each end and rounded corners. The entrance goes on the side facing the nearest road; on a long side the stalls in front of it are left out so it opens onto an aisle, and a driveway runs straight to the closest point on the road, meeting it at a T (splitting the road there, or using a node within 8 m). Moving or resizing rebuilds the driveway and rejoins the road it split. Painted lots load as their bounding box. Boreal's lot is redrawn as a 140 × 90 m rectangle along the road, 25 m off it, with 508 stalls (the old one had 505); its two stubs are gone.
4. Planned separately: **Surfaces**, asphalt, gravel, or dirt per lot, in [[Lot Surfaces]].
5. **Later.** Employees driving in to an employee lot, and their commutes (Staff in [[Next Steps]]); better traffic (merging, turning lanes, signals); road closures (*for* [[Alta]]); trains (*for* [[Zermatt]]); parking choice weighted by distance to the lifts.

## How it works

Code: `internal/sim/traffic.go`, `internal/world/car.go`.

- **Lanes.** Each direction of each road edge is a lane: the road's centre spline offset 1.5 m to the right. Speeds: 35 mph on the road, 15 mph on a leg touching a lot's driveway, 10 mph in a lot, 5 mph easing into a stall.
- **Following.** Cars never overtake and keep at least a car length plus 2.5 m to the car ahead, slow enough to stop behind it if it brakes (0.8 s reaction), which gives about a 1.2 s gap at speed.
- **Junctions and lot entrances.** Nodes where three or more roads meet, and lot entrances, are taken in turns. A car asks as it nears and stops 4 m short until it's let through, then holds the node until 5 m past it. Movements that share a way in (they split) or a way out (they merge, keeping their gap on the lane out) go through together as a stream; a crossing movement waits for the node to empty, and once it has waited 3 s the stream stops admitting new cars. A car is only let through when there's room past the node, so it never stops inside a junction and blocks it ("don't block the box"; without this, leaving cars filling the stub to a junction gridlocked the lot). A car that has waited 20 s goes anyway once it has room.
- **Lot entrances.** A lot's driveway node when a road reaches it, plus any road dead end within 6 m of the lot.
- **Lots** (`internal/world/parking.go`): a rectangle (centre, rotation, `LotSize`), with its cells (for grading, plowing and overlap), entrance (`Gate`), stalls and aisles derived from it. Stalls are 2.5 × 5 m in double-loaded rows facing 6 m aisles, with a 6 m cross aisle at each end; the lot is at least 11 m wide (one row and its aisle) and at most 400 m a side. The entrance stays off the 4 m rounded corners. Grading cuts and fills the lot to a plane (at most 5%) and ramps the driveway straight from the lot to the road; embankments leave every cell within a road's half width plus a cell untouched, so a lot never cuts into a road beside it.
- **In the lot**, a car follows the aisles from the entrance to its stall (in to the nearest aisle or the end aisle, along the end aisle to its own, along that to the stall, and in nose first); cars in a lot don't see each other.
- **No road, no entries.** A lot no road reaches, and maps without entries (testbeds, older scenarios), still work: the car appears in a free stall and vanishes when it leaves.
- **Vehicles.** A car is a sedan, mini SUV, SUV, jeep or van (`world.CarKind`), maybe with a ski rack or roof box (`world.CarRoof`), rolled from its ID in `world.RollCar`: a van when the carload is five to seven, otherwise the bigger kinds likelier for three or four. Each kind seats four except the van (seven), so a group comes in as few vehicles as hold it. Every kind fits a 2.5 × 5 m stall (4.1–4.9 m long). Models: `models-src/car_*.scad` on `lib/car_kit.scad`; the renderer draws each kind and roof load from its own batch, with the paint from a palette of common car colours.
- **Demand.** Guests in arriving cars count toward the resort's occupancy. The per-guest parking share in the price factor is the fee ÷ 2.4, the mean carload.
- **Cost.** Cars step once per frame, in steps of up to 0.5 s of sim time, not with the guests' 1/30 s substeps. The road network is rebuilt when a node, edge, or lot changes.

## Open questions

- How many cars at once before performance matters: settled for now, about 4,000 cars cost about 0.4 ms a tick (step 2).
- Throughput: one lot entrance takes about a car every 1.5 s when nothing leaves (about 2,400 an hour on Boreal); with cars leaving as fast as they arrive it drops to about one every 6 s. More entrances per lot, or turning lanes, may be needed for very big resorts.

## Log

- 2026-10-06: Planned with the user after looking at Boreal's lot and roads: simulated traffic up front, rectangular extendable lots, asphalt, gravel, and dirt, and editor-controlled entry and exit points tied to demand.
- 2026-10-06: Decided: carloads of one to four; 35 mph on the two-lane road, 5–15 mph in lots; no import suggestions for entry points.
- 2026-10-06: Step 1: named entries, each with its own guest pool (the user's call, replacing a share of arrivals); every guest belongs to one entry and arrives and leaves by it. The goal for step 2 is thousands of cars.
- 2026-10-06: Boreal's road ends became entries: I-80 west, 80,000 guests; I-80 east, 20,000.
- 2026-10-06: Step 2: cars drive in from their entry, park, and drive home when the carload is aboard, with lanes, following, junctions taken in turns (streams of the same movement, don't block the box), and road dead ends touching a lot counted as its entrances. Painted lots' `CurrentCars` is gone; the lot popup counts parked cars.
- 2026-10-06: Step 3: rectangular lots drawn and resized by dragging, turned to the nearest road, with an entrance facing it and a driveway that meets it at a T; cars follow the aisles. Found along the way: lot embankments cut into roads beside them, and the road strip (sampled between cell centres) sank under the terrain mesh beside a cut; roads are now protected from embankments and drawn on the mesh's own height. Movements that merge or split now share a junction, which roughly doubled entrance throughput.
- 2026-10-06: Surfaces split out into [[Lot Surfaces]], to build later; the rest of the plan's later items stay here.
- 2026-10-06: Added employee cars and lots to the later items (Staff in [[Next Steps]]).
- 2026-10-06: R with the Parking tool over an existing lot turns it about its centre (free; refused if it would overlap), rebuilding the driveway and regrading; parked cars are re-seated in their stalls after any turn or resize.
- 2026-10-06: Per the user: built lots don't turn or move (demolish and rebuild), so turning an existing lot came out again. Drawing or resizing a lot now leaves a ghost (translucent asphalt and stall lines, red when it won't fit) that R turns and edge drags adjust; clicking inside it or Enter builds it, and only then is the ground graded and plowed. Esc throws it away.
- 2026-10-06: A new lot's ghost follows the mouse after it's drawn (move to place, R turns it, click builds, Esc and drag again for another size); its edges no longer resize it. Resizing a built lot stays a fixed ghost confirmed by a click.
- 2026-10-09: Car kinds: sedan, mini SUV, SUV, jeep and van, with ski racks and roof boxes, in common car colours; vans seat seven, so groups of five to seven come in one. Buses and RVs noted in [[Large Vehicles]].

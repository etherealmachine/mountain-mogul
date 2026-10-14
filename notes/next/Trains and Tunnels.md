---
title: Trains and Tunnels
kind: plan
status: partial
---

# Trains and Tunnels

Rail lines that bring guests in with no parking, in three styles, a funicular lift on rails, and tunnels for rail, roads and footpaths through the hills. Mostly because they should look great. Touches [[Transit]], [[Parking and Roads]], [[Lifts]], [[Demand]], [[Terrain]] and [[Model Pipeline]]. *For* [[Zermatt]].

## What's there today

Surveyed 2026-10-14:

- **Roads** are a node and edge graph, curved through Catmull-Rom chains, built click by click with snapping, $200 plus $5 a metre; no slope limit; they don't grade the ground (they lie on it), and bridges over water are built automatically. Footpaths are their own node lists.
- **Arrival** is a group of guests and the road entry they came by: demand hands each group to `spawnCar`, which parks and then calls `spawnGuestAt` at the stall. A train can call the same at a station.
- **The tram** is two cabins shuttling on a straight line (`tramAt`): a funicular is the same with the cars on rails on the ground.
- **The ground** is a heightmap: no holes, so a tunnel can't be seen into. `GroundPatch` cuts and fills it.
- **Vehicles turn but don't tilt**: the dynamic shader rotates by heading alone, so a car on a grade sits level.

## Decisions

From the user, 2026-10-14:

- **Three train styles that play differently**: an old steam engine, a modern German-style electric, and streamlined high-speed rail, each its own line type with its own cost, speed and capacity.
- **The funicular is a lift type**: two counterbalanced cars on a rail up the slope, carrying skiers up, through a tunnel where the line goes under the hill.
- **Tunnels show as portals**, the line's vehicles hidden between them, its course a faint dashed line on the surface.
- **Tunnels carry rail, roads and footpaths.**

## How it works (proposed)

- **Styles**:
  - *Steam*: a tank engine and three wooden coaches, about 150 riders; 40 km/h; cheap track ($600 a metre); smoke and steam; grades to 4%; a draw in itself (a small demand bonus, "rode the steam train" a highlight).
  - *Electric*: a German-style regional unit (a locomotive and double-deck coaches, or a multiple unit), about 400 riders; 100 km/h; $1,500 a metre with overhead wire on masts; grades to 4%.
  - *High-speed*: a streamlined trainset, about 600 riders; 250 km/h on the map's long straights; $4,000 a metre; grades to 3.5% and wide curves.
- **Rail lines**: built like roads, click by click from a rail entry at the map edge (placed in the editor, like road entries) to a station; each line has one style. The rail bed is graded into the ground within its grade limit (cuts and fills), with ballast, sleepers and rails drawn on it and, for electric and high-speed, masts and wire.
- **Stations**: a platform building as long as the style's train, with a shelter; arriving guests get off and walk to the base, leaving ones wait on the platform.
- **Service**: trains run to a timetable (every 30 or 60 clock minutes, set at the station), each bringing the groups demand has sent by rail from its entry and taking home those waiting. A group comes by train or car by how it reaches the resort: a share of each entry's arrivals goes by rail when a line serves it, more with a better train. Each train costs to run.
- **Tunnels**: where a line, road or path would cut deeper than about 12 m into a hill, it goes into a tunnel instead (portal models at each end, $8,000 a metre); its vehicles and walkers are hidden inside, and the course is a dashed line on the surface.
- **The funicular** (`LiftFunicular`): two cars of stepped compartments, about 100 riders each, counterbalanced on one track with a passing loop in the middle, up to 45°; the cars pitch with the slope; tunnelled where the line runs under the ground.
- **Pitch**: vehicles get a pitch as well as a heading, so trains on grades and funicular cars tilt.

## Built: the models

All along the track (+X, the A end), from the top of the rail (`models-src/lib/rail_kit.py`: bogies, wheelsets, buffers, bodies, window bands, roof sections, pantographs):

- **Steam**: `steam_loco.py`, a green tank engine with a black boiler, tall chimney, brass dome and three red driving wheels a side (9.6 m); `steam_coach.py`, a maroon and cream clerestory coach with open end platforms (12.4 m).
- **Electric**: `electric_loco.py`, a traffic-red DB class 146 with sloped cab fronts and two pantographs (18.9 m); `electric_coach.py`, a red double-decker with its lower deck dropped between the bogies (26.8 m).
- **High-speed**: `hsr_power.py`, a white power car with a long tapering nose, wraparound windscreen and red stripe (20.5 m, one at each end); `hsr_coach.py`, a close-coupled coach with a continuous window band (24.8 m).
- **Funicular**: `funicular_car.py`, four red and cream compartments stepped on an 11 m chassis, level on a 20° line (the renderer pitches it with its line); each compartment pivots on its uphill floor corner, a red skirt under its raised downhill end; a hundred standing places, twenty-five a compartment.
- **Portals**: `lib/portal_kit.py`, a stone face round an arched opening, dark inside, with wing walls: `tunnel_portal_rail.py` (5 × 6.8 m), `_road.py` (9 × 6), `_path.py` (3.2 × 3.2).
- `tools/modelview/view.py` renders OBJs to a PNG sheet for a look without the game.

None are in the demo. The station waits for step 2, where its platform's length is set by the style.

## Built: rail lines and stations

- **Styles** (`world/rail.go`, `RailSpec`): steam 11 m/s, 150 riders, $600 a metre, a $150,000 station, $300 a trip, grades to 4%, an engine and 3 coaches; electric 28 m/s, 400 riders, $1,500 a metre, $300,000, $600 a trip, 4%, an engine and 4 double-deckers, wired; high-speed 70 m/s, 600 riders, $4,000 a metre, $600,000, $1,200 a trip, 3.5%, a power car each end and 6 coaches, wired.
- **The Rail tool**: three buttons in the Transport menu, one a style. The first click must be within 25 m of the map's edge (no editor entries: a line can come in anywhere along the edge); each click adds a point; clicking the last point again builds it, its station at that end; Esc drops the line. A ghost of the laid-out line follows the mouse, red when it's too short (its platform and 60 m) or unaffordable, with the cost by the cursor.
- **Laying out** (`RailLine.LayOut`): a Catmull-Rom curve through the points, sampled every 2 m; the track's height follows the ground plus the ballast, relaxed and clamped to the style's grade, level along the platform at the station end.
- **The bed** (`GradeRailBed`): the ground within 3 m of the track cut or filled to the ballast's foot, shoulders banked 1.5 to 1 out to the ground (up to 25 m), trees cleared 9 m either side; the bed's cells are plowed of snow when it's built and at each day's rollover (`PlowRails`).
- **Drawn** (`render/rail_mesh.go`, a static mesh a line): ballast with sloping sides, sleepers every 0.7 m, two rails, the platform (as long as the train and 10 m) with a yellow edge, and for wired lines masts every 45 m and the wire; the station house (`rail_station.py`) beside the platform's middle. Saved as each line's style and points, laid out again on load.
- On the Goals Test, an electric line from the west edge to near the base: 698 m, $1,347,000.

## Built: trains running

- **Timetable** (`sim/trains.go`): a line's one train sets off from the map's edge every Interval clock minutes (60 by default; 15 to 240 in the station's popup) from two hours before the lifts open to half an hour after they close, runs in at its style's speed, brakes at 0.6 m/s² to stop by the platform's end, stands four clock minutes, and pulls away at 0.5 m/s² back off the map. Each trip costs its style's TripCost, the day's "Trains" cost line.
- **Riders**: each demand poll books a share of its arriving groups on a line's next train instead of a car (steam 12%, electric 30%, high-speed 45%, at most 80% together; all of them with no car park), picked by those shares. The train lets off whole groups up to its capacity on the platform; the rest take the next. Their "lot" is the station (`BuildingStation`, a building with no tiles at the platform's middle): they leave by walking back to it (`IsDepartPoint`). Nobody's booked overnight, and a save made with guests booked sends them home.
- **Drawn** (`render/trains.go`): the engine and coaches along the track, the engine leading whichever way it's running (a high-speed set's second power car at the tail facing back), pitched with the grade: dynamic instances now have a pitch, in the dynamic and shadow shaders.
- **Station popup**: the train's riders, speed and trip cost, the interval, where the train is, how many are booked and how many came by train.
- On the Goals Test with an electric line: of 1,041 visitors in a day, about 250 came by train, every hourly train clearing its bookings; all left by evening.

## Built: rail tunnels

- Where a line's grade-held track runs more than 12 m under the ground, it tunnels (`world/tunnels.go`): runs within 40 m of each other join, and a tunnel under 30 m is cut instead. The hill isn't graded over a tunnel; its track, masts and wire aren't drawn, nor its train; a portal (`tunnel_portal_rail.py`) stands at each mouth facing out, and a dashed line on the snow marks the course. A tunnel costs $8,000 a metre in place of the track's price (`RailLineCost` now prices the laid-out line, so the tool's cost preview counts tunnels).
- The bed is graded to the nearest point on the track's segments, not its sample points (the banks were toothed), and plowed like the roads: bare within 6 m of the track, the snow back over the next 6 m (cell-by-cell clearing left a sawtooth).
- Each line's track heights and tunnel spans are saved: laying a line out again over the ground it graded moved its tunnels (a portal ended up 25 m inside the uncut hill).
- On the Goals Test a line from the north-east corner to the base runs 1,064 m in a tunnel under the mountain.
- **Roads and footpaths** don't tunnel yet: they lie on the ground with no grade limit, so they'd first need a profile of their own (held to a grade, graded into the ground) for a tunnel to come from.

## Built: the funicular

- **The lift** (`LiftFunicular`, `world/funicular.go`): in the Lifts menu after the tram. Two 100-rider cars on the tram's shuttle (`Shuttle()` is the tram or the funicular: two places half a loop apart, a round trip with a 45 s dwell at each end, the whole line boarding at once), on one track: `tramAt` puts them on the line, swinging 1.8 m out each way on a passing loop in the middle (30 m either side of it, easing out over 20 m). 10 m/s; $2.5M for the stations, $3,000 a metre, $5,000 a day; at least 150 m; up to 45°, measured end to end (the ground between is cut, filled or tunnelled). No towers, no cable; it never holds for wind, nor for a bare base (it's on rails).
- **The track** (`FunicularTrack`, not saved, laid out again from the ground): a straight line from the base to the top at one grade, ballast over the ground at each end, the stations on the slope too; graded like a railway's bed (`GradeRailBed`) instead of levelling station aprons, plowed with the railways (`PlowRails`), tunnelled where it runs more than 12 m under (portals at the mouths, cars and their riders hidden inside, `Underground`). The cars ride it (`CableYAt`) pitched with it (`PlacePitch`), riders' places pitched with them (`seatWorldPos`).
- **Drawn** (`render/funicular.go`, a mesh a lift, rebuilt when its track is): ballast, sleepers and a pair of rails, two pairs on the loop; at each end the station: a platform beside the track on the riders' side stepping down the slope a metre at a time, its yellow edge, a roof on posts sloping with the track, buffers at the track's end. Riders board and leave on the line, at the car.
- On the Goals Test, a 770 m funicular beside Lift1 climbs 145 m at 10.7°, its cars passing in the middle.
- Fixed on the way: the railway's ballast, rails and tunnel dashes were wound facing down, so lit from below and dark.

## Steps

1. Done: **The models**: steam engine and coaches, the electric unit, the high-speed set, the funicular car, a station, a tunnel portal; checked on screen.
2. Done: **Rail lines and stations**: the Rail tool with grade limits, the graded rail bed and its look, stations.
3. Done: **Trains running**: timetables, consists following the track with pitch, guests arriving and leaving by train, costs.
4. Rail done; roads and footpaths left: **Tunnels**: on rail, roads and footpaths: deep cuts become tunnels, portals, hidden vehicles, the dashed course.
5. Done: **The funicular**: the lift type on the tram's shuttle, on rails, tunnelled.
6. **Balance**, and a look at it all on Zermatt-like ground.

## Open questions

- Whether a rack railway (the Gornergrat's cog railway, grades to 20%) is a fourth style.
- How a line from the map edge reaches a resort with no flat ground for a station.

## Log

- 2026-10-14: Surveyed and planned with the user, ranked first.
- 2026-10-14: Step 1's models built and looked at on a sheet (`tools/modelview`).
- 2026-10-14: Step 2 built (the Rail tool, laying out, the bed, the look, stations). The first look had the ballast and rails buried in snow: the bed is now plowed.
- 2026-10-14: Step 3 built (timetables, booking riders, arrivals and departures by the station, consists with pitch, the station popup).
- 2026-10-14: Rail tunnels built (step 4's rail half); roads and footpaths need profiles first.
- 2026-10-14: Step 5 built (the funicular: the lift type, its straight graded track with a passing loop, stepped platforms, pitched cars, tunnels). The car model restepped for 20° after the first look had its compartments hanging through the chassis.

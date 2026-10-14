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
- **Funicular**: `funicular_car.py`, four red and cream compartments stepped on an 11 m chassis, level on a 30° line (the renderer pitches it with its line).
- **Portals**: `lib/portal_kit.py`, a stone face round an arched opening, dark inside, with wing walls: `tunnel_portal_rail.py` (5 × 6.8 m), `_road.py` (9 × 6), `_path.py` (3.2 × 3.2).
- `tools/modelview/view.py` renders OBJs to a PNG sheet for a look without the game.

None are in the demo. The station waits for step 2, where its platform's length is set by the style.

## Steps

1. Done but for the station: **The models**: steam engine and coaches, the electric unit, the high-speed set, the funicular car, a station, a tunnel portal; checked on screen.
2. **Rail lines and stations**: the rail entry in the editor, the Rail tool with grade limits, the graded rail bed and its look, stations.
3. **Trains running**: timetables, consists following the track with pitch, guests arriving and leaving by train, costs.
4. **Tunnels**: on rail, roads and footpaths: deep cuts become tunnels, portals, hidden vehicles, the dashed course.
5. **The funicular**: the lift type on the tram's shuttle, on rails, tunnelled.
6. **Balance**, and a look at it all on Zermatt-like ground.

## Open questions

- Whether a rack railway (the Gornergrat's cog railway, grades to 20%) is a fourth style.
- How a line from the map edge reaches a resort with no flat ground for a station.

## Log

- 2026-10-14: Surveyed and planned with the user, ranked first.
- 2026-10-14: Step 1's models built and looked at on a sheet (`tools/modelview`).

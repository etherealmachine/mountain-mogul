---
title: Architecture Spec
kind: spec
---

# Architecture

## Entry point

`main.go` initialises GLFW/OpenGL, creates `engine.App`, and pushes the
first scene onto the stack. Key flags: `-testbed <name>`, `-headless`,
`-screenshot`, `-profile`, `-trace`.

## Package dependency flow

```
main.go
  └─ engine.App          (framework, window, scene stack)
       └─ scene.Scenario (main gameplay scene)
            ├─ sim.Simulation   (logic tick)
            │    └─ world.World (all persistent state)
            └─ render.Renderer  (GPU; reads world read-only)
```

`internal/ai` is a leaf package: it holds the AI types embedded in
`world.Guest` so `world` doesn't import `sim` and `sim` doesn't import
`world` transitively through the planner. `engine.Scene` is an interface
so `engine.App` can hold any scene without importing `scene`.

---

## Packages

### `internal/engine`
Core framework. Window creation, input, and the scene-stack update/render
loop. Scenes implement `Init / Update / Render / Destroy`.

**Key types:** `App`, `Input`

---

### `internal/world`
Owns all persistent simulation state. No logic — only data and
domain helpers (entity lookup, coordinate conversion, ID allocation).

**Key types:**

| Type | Role |
|---|---|
| `World` | Master container. Holds `Terrain`, guest pool, buildings, lifts, snowcats, roads, cash balance. |
| `Terrain` | Height-map grid. Each `Cell` stores `GroundElevation`, `SnowDepth`, `Grooming`, `Packed`, `Ice`, `MogulSize`, `TreeDensity`, `Passable`. |
| `SurfaceDetail` | 1 m-resolution RGBA8 texture (5× cell grid). R = skier tracks, G = tree wells, B = groom-edge mask. See [[Snow Spec]]. |
| `Guest` | Resort visitor. Identity + career stats (`Visits`, `LastRating`); on-mountain transient state (position, speed, energy, fun, fear, `Plan`, `Balance`); `Thoughts` ring for RCT-style feedback. |
| `Lift` | Cable lift. Base/top positions, speed, per-ride fare (heli only; cable lifts are covered by the day ticket), `[]Chair` loop, queue of waiting guests. |
| `Building` | Placed structure (lodge, shed, parking lot). Sheds own snowcats and a painted grooming route. Parking lots and service buildings are *painted*: `Cells` is the footprint. A service building (`BuildingLodge`) gives every cell a `Service` in `Tiles`: Lounge (rest), Food court (seats = 10 per tile; meals at `MealPrice` restore hunger and book `RevenueFood`), Bar (drinks at `DrinkPrice`, `RevenueBar`) or Tickets (day tickets, passes, and the resort-wide controls in its popup). `FloorY` is fixed by the first tile. Doors are automatic (`RefreshDoors`, rerun by `RebuildTrailGraph`): each 4-connected run of one service gets one door on an outside wall that opens onto free walkable ground, on the side nearest the parking lot for Tickets and the nearest lift base otherwise. Guests treat the whole building as one place and walk to the door of the service they want. `world/lodge_shell.go` resolves a shell into tile instances on a 2.5 m half-cell grid: walls, windows, doors and glazing on outside edges, corners by marching squares, and a hip roof from each tile's Chebyshev distance to the outside. Older point-placed lodges convert to a 4×3 block of Lounge tiles, and standalone bars and ticket offices to Bar or Tickets tiles over their footprint (`ConvertLegacyBuilding`, on placement and on load). |
| `Snowcat` | Grooming machine. Drives to route cells, applies corduroy (raises `Packed`, lowers `SnowDepth`). Parked while the lifts run; after closing, each active cat makes one pass of its section per night if any snow-covered cell in it is below 90% groomed. |
| `RoadNode / RoadEdge` | Road graph vertices and segments. Nodes typed: freestanding, edge-connection, parking driveway, auto-intersection. |
| `History` | Daily ring of resort stats (guests on mountain, arrivals, departures, cash, revenue by `RevenueKind`, costs by `CostKind`). Feeds the in-game charts and the nightly report. |
| `EventLog` | `World.Events`: bounded ring (256) of `Event`s — kind, sim time, message, optional XZ position + entity ID. Written by the sim (avalanche, patrol rescue, lift holds, day recap) and by the scene for player actions (builds, lift open/close) via `Simulation.Log*`. Shown in the left-side event panel (top-bar flag button); clicking a positioned event centres the camera there. |
| `GuestPool` | Master roster of all guests (on-mountain + departed). Tracks per-guest career across visits. |

---

### `internal/ai`
Leaf package — persistent AI types embedded in `world.Guest`. No logic;
exists solely to break the `world` ↔ `sim` import cycle.

**Key types:** `GuestTraits` (skill, comfort speed/slope, aggression),
`Plan` (L0 GOAP output: steps, current goal, target position),
`PlanAction`, `Sense` (per-tick perception snapshot for HUD),
`Thought` (ring-buffered RCT-style thoughts), `GuestEvent`

---

### `internal/sim`
All game logic. Ticks guests (GOAP planner + L1–L3 physics controller),
lifts, snowcats, and the demand/rating system.

**Key types:**

| Type | Role |
|---|---|
| `Simulation` | Central tick driver. Holds `World`, pathfinder, planner, demand system, RNG seed, time scale. |
| `DemandSystem` | Resort rating + guest arrival. Bernoulli roll per guest every 30 sim-seconds; daily EMA from departure satisfaction. |
| `Pathfinder` | A* grid navigation for walk segments between buildings and lift bases. |

**Calendar** (`calendar.go`): `DateAt(start, simTime)` is `start` plus
one day per `secondsPerSimDay` (4320 s: 24 clock hours of
`world.SimSecondsPerHour` = 180 s, about 5 real minutes for a 9:00–16:00
ski day at 4×). `HourOfDay(simTime)` is the clock hour: local time when the map has a
time zone, solar time otherwise; `sun.go`'s `Site` (from the world's
bounds and zone, or 45°N) gives the sun's direction and sunrise/sunset,
`temperature.go` the hourly air temperature (the day's low at sunrise,
high at 14:30, cooling toward the next day's forecast low), and the
renderer lights the scene from `Simulation.Sun()` (moonlight at night).
Terrain casts shadows through `world.HorizonMap` (per-cell horizon
angles in 16 directions, recomputed lazily after `RecomputeSlopes`): the
renderer turns it into a per-cell sun-visibility texture for the current
key light, and hourly melt uses the same visibility. Objects (trees,
buildings, lifts, chairs, vehicles, guests) cast through a 4096² directional
shadow map (`render/shadow_map.go`) fitted to the visible ground each frame
and texel-snapped so it doesn't shimmer; every lit shader multiplies the key
light by `keyLightVisibility()` in `lighting.glsl`. Object shadows are
visual only; melt ignores them.
Lifts turn between `World.OpenHour` and `CloseHour` (default 9–16, set in
a Tickets building's popup). Saves store `day_sec`; older saves (240 s days)
have their absolute sim times rescaled on load. `Simulation.DateAt(simTime)`
passes the sim's `World.StartDate`. `StartDate` is the date SimTime 0
maps to: `world.DefaultStartDate` (Nov 25, 2026) unless the scenario
sets one with the editor's **Start date** strip under the top bar. It
is saved as `start_date` ("2006-01-02"), so a New Game from a scenario
begins on that date with SimTime 0. The calendar runs through the
whole year; there is no off-season jump. Whether the resort is open is
the player's call, not the calendar's: `World.ResortOpen`, flipped by
`Simulation.SetResortOpen` from a Tickets building's popup (`resort.go`).
The day rollover (`maybeSampleHistory`) charges `OperatingCosts` on
days the resort was open at any point and `StandbyCosts` otherwise
(both broken down by `CostKind`; the scene opens the day's profit/loss
report from `OnDayRollover`, `scene/day_report.go`),
bills credit interest at each month end (`credit.go`), and advances the
weather chain, which samples all twelve month profiles by date. The
sim also samples the chain once at creation, so day 1's weather matches
the start month.

Skiing is the hot path: `tickSkier` runs the full L1–L3 pipeline
(perception → steering → physics integration → balance/fall check) once
per guest per tick. See [[Guests Spec]].

---

### `internal/render`
GPU rendering pipeline. Reads `world.World` read-only; owns all OpenGL
objects.

**Key types:**

| Type | Role |
|---|---|
| `Renderer` | Coordinates all passes (terrain, static, dynamic, UI). Holds shaders, camera, instanced batches, `SnowSurfaceTex`. |
| `Camera` | 45° isometric ortho + optional first-person. Manages view/projection matrices. |
| `SceneResources` | GPU state tied to the current world: terrain VBO, static batch, scene textures. Rebuilt on world change. |
| `Shader` | GPU program wrapper (compile, bind, uniforms). |
| `Font` | Glyph-based text via a packed texture atlas. |

Lodge shells draw from a 15-piece tile kit (`models-src/lodge_*.scad`,
shared parameters in `models-src/lib/lodge_kit.scad`) loaded into static
batches at `MeshLodgeTileBase + ShellTileKind`; `RebuildStaticBatch`
adds each shell's tiles with a per-lodge wall and roof tint picked by
`StyleSeed`.

Terrain overlay bitmask: contour lines, slope, snow depth, grooming,
packed, ice, moguls, bump normals, surface detail.

---

### `internal/scene`
Game scenes implementing `engine.Scene`. Handles main menu, gameplay,
terrain import, editor tools, and all placement flows.

**Key types:** `Scenario` (main gameplay scene — owns simulation, renderer,
toolbar, top bar, charts, debug panels), `StartMenu`, `ScenarioPicker`,
`TerrainImportScene`, `SaveListScene`, `EscapeMenu`

Tool modes in `Scenario`: lift placement (2-click), building placement,
road placement, terrain brushes (raise/lower/glade/plant), snowcat route
painting, parking-lot painting, and the service tool (`service_tools.go`).
The Amenities submenu picks a service; the cursor ray is tested against
each tile's box (`pickService`). Clicking the ground starts a building
(or joins one beside it), clicking a wall adds a tile on that side,
clicking a roof switches that tile's service, and a right click that
doesn't pan removes a tile. New ground is graded to the building's
`FloorY`. A ghost tile (`Renderer.SetShellGhost`) previews the click,
red when it can't go ahead. While a building's popup is open it renders
as a cutaway (`Renderer.SetLodgeCutaway`): no roof, walls squashed to
~1.25 m, and the cell overlay colours its floor by service and marks
the doors.

---

### `internal/save`
JSON serialization for save/load. Stable `json` tags insulate the format
from internal renames.

**Key types:** `ScenarioData` (full world snapshot), `CellData`,
`BuildingData`, `LiftData`, `GuestData`, `SnowcatData`, `HistoryData`,
`EventData`, `CameraData`

---

### `internal/geo`
Real-world terrain import. Fetches elevation tiles from AWS Terrain Tiles
(Terrarium, zoom 14) and, where it exists, USGS 3DEP 1 m lidar (cloud-optimized
GeoTIFFs read by range request), samples them onto the cell grid and the 1.25 m
detail lattice, geocodes lat/lon.

**Key functions:** `ImportTerrain`, `FetchGrid`, `OpenLidar`, `Geocode`, `Preview`

---

### `internal/ui`
In-game HUD widgets. All drawing via the renderer's UI pass (batched quads).

**Key types:** `Window`, `Button`, `VSlider`, `HSlider`, `TextInput`,
`TopBar`, `MenuBar`, `OverlayPanel`, `ChartWindow`

---

## `assets/`

```
assets/
  icons/       PNG icons for toolbar/UI buttons
  models/      OBJ meshes
    chair.obj  chair_quad.obj  snowcat.obj  car.obj
    building.obj  shed.obj  parking.obj
    tower.obj  lift_station.obj
    tree.obj  tree2.obj  tree3.obj  rock.obj  stump.obj
  scenarios/   Bundled save files (e.g. boreal.save)
  shaders/
    terrain.vert / terrain.frag   (main terrain pass)
    static.vert  / static.frag    (instanced buildings, trees)
    dynamic.vert / dynamic.frag   (skiers, snowcats, chairs)
    ui.vert      / ui.frag        (HUD quads)
    debug.vert   / debug.frag     (overlay / perception cone)
    lighting.glsl                 (shared PBR lighting)
```

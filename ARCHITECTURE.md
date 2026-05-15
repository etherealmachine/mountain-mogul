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
| `SurfaceDetail` | 1 m-resolution RGBA8 texture (5× cell grid). R = skier tracks, G = tree wells, B = groom-edge mask. See [SNOW.md](SNOW.md). |
| `Guest` | Resort visitor. Identity + career stats (`Visits`, `LastRating`); on-mountain transient state (position, speed, energy, fun, fear, `Plan`, `Balance`); `Thoughts` ring for RCT-style feedback. |
| `Lift` | Cable lift. Base/top positions, speed, ticket price, `[]Chair` loop, queue of waiting guests. |
| `Building` | Placed structure (lodge, shed, parking lot). Sheds own snowcats and a painted grooming route. |
| `Snowcat` | Grooming machine. Drives to route cells, applies corduroy (raises `Packed`, lowers `SnowDepth`). |
| `RoadNode / RoadEdge` | Road graph vertices and segments. Nodes typed: freestanding, edge-connection, parking driveway, auto-intersection. |
| `History` | Daily ring of resort stats (guests on mountain, arrivals, departures, cash). Feeds the in-game charts. |
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

Skiing is the hot path: `tickSkier` runs the full L1–L3 pipeline
(perception → steering → physics integration → balance/fall check) once
per guest per tick. See [GUESTS.md](GUESTS.md).

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
painting.

---

### `internal/save`
JSON serialization for save/load. Stable `json` tags insulate the format
from internal renames.

**Key types:** `ScenarioData` (full world snapshot), `CellData`,
`BuildingData`, `LiftData`, `GuestData`, `SnowcatData`, `HistoryData`,
`CameraData`

---

### `internal/geo`
Real-world terrain import. Fetches elevation tiles from AWS Terrain Tiles
(Terrarium, zoom 14), resamples to the target grid, geocodes lat/lon.

**Key functions:** `FetchGrid`, `ResampleToGrid`, `Geocode`, `Preview`

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
    skier.obj  chair.obj  chair_quad.obj  snowcat.obj  car.obj
    building.obj  shed.obj  parking.obj
    tower.obj  lift_station.obj
    tree.obj  tree2.obj  tree3.obj  rock.obj  stump.obj
  scenarios/   Bundled save files (e.g. tutorial.save)
  shaders/
    terrain.vert / terrain.frag   (main terrain pass)
    static.vert  / static.frag    (instanced buildings, trees)
    dynamic.vert / dynamic.frag   (skiers, snowcats, chairs)
    ui.vert      / ui.frag        (HUD quads)
    debug.vert   / debug.frag     (overlay / perception cone)
    lighting.glsl                 (shared PBR lighting)
```

# Mountain Mogul — MVP Architecture Plan

---

## Tech Stack

- **Language:** Go 1.25
- **Windowing & Input:** `github.com/go-gl/glfw/v3.3/glfw`
- **OpenGL bindings:** `github.com/go-gl/gl` (4.1 core — macOS ceiling)
- **Math:** `github.com/go-gl/mathgl`
- **Geo import:** `github.com/flopp/go-staticmaps`, `github.com/tkrajina/gpxgo`, `github.com/golang/geo`
- **Save format:** JSON via standard `encoding/json`

No game engine. No physics library. No UI framework.

---

## Tutorial Scenario: Boreal Mountain Resort

The first tutorial is based on Boreal Mountain Resort, Donner Pass, California.

| Stat | Value |
|---|---|
| Skiable area | 380 acres |
| Vertical drop | 500 ft (152 m) |
| Longest run | 1 mile (1,609 m) |
| Trails | 41 (30% green, 55% blue, 15% black) |

**Grid:** 128 x 256 cells at 10 m/cell (1,280 m E-W x 2,560 m N-S fall-line). Covers the ~1,000 m x 1,600 m Boreal footprint with buffer for base areas.

---

## Folder & File Layout

```
mountain-mogul/
├── main.go
├── go.mod
├── go.sum
├── DESIGN.md                               # full game design document
│
├── assets/
│   ├── shaders/
│   │   ├── lighting.glsl                   # shared flat-shading snippet, prepended at load time
│   │   ├── terrain.vert / terrain.frag     # heightmap mesh; frag shader handles brush ring overlay
│   │   ├── static.vert  / static.frag      # instanced OBJ models, placement-time buffer
│   │   ├── dynamic.vert / dynamic.frag     # instanced OBJ models, per-frame buffer + animation
│   │   └── ui.vert      / ui.frag          # screen-space UI quads
│   ├── models/                             # OBJ + texture pairs
│   │   ├── tree.obj  / tree2.obj / tree3.obj
│   │   ├── EvergreenTexture.png
│   │   ├── PineTexture.png
│   │   └── PineTree1Snowy.mtl / PineTree2Snowy.mtl / PineTree3Snowy.mtl
│   └── scenarios/
│       └── tutorial.json                   # Boreal tutorial resort
│
└── internal/
    ├── engine/
    │   ├── app.go          # App struct, main loop, scene stack wiring
    │   ├── window.go       # GLFW window + OpenGL context init
    │   ├── input.go        # per-frame keyboard/mouse snapshot
    │   └── scene_iface.go  # Scene interface definition
    │
    ├── geo/
    │   ├── geocode.go      # converts place names / coords to bounding box
    │   ├── elevation.go    # fetches real-world heightmap data for a bounding box
    │   ├── resample.go     # resamples heightmap to the world grid resolution
    │   └── preview.go      # renders a 2D map preview for the terrain import UI
    │
    ├── render/
    │   ├── renderer.go     # top-level draw coordination, OpenGL state; SetBrush/ClearBrush
    │   ├── shader.go       # compile, link, uniform helpers; prepends lighting.glsl
    │   ├── mesh.go         # VAO/VBO creation, mesh ID constants
    │   ├── obj.go          # OBJ + MTL loader -> Mesh; loads referenced PNG textures
    │   ├── texture.go      # PNG loading via image/png; uploads to OpenGL
    │   ├── batch.go        # instanced draw; StaticInstance and DynamicInstance types
    │   ├── camera.go       # fixed-angle orthographic camera with pan, zoom, ray-casting
    │   └── font.go         # bitmap font renderer
    │
    ├── ui/
    │   ├── menubar.go      # horizontal tool bar across top of screen
    │   └── button.go       # rectangular clickable element with label
    │
    ├── scene/
    │   ├── scene.go        # Scene interface (also defined in engine/scene_iface.go)
    │   ├── startmenu.go    # Start Game, Load Save, Editor, Settings, Exit
    │   ├── scenario.go     # main gameplay scene; DDA screenToCell
    │   ├── editor.go       # scenario editor scene
    │   └── terrain_import.go  # real-world terrain import flow (geocode -> preview -> confirm)
    │
    ├── world/
    │   ├── world.go        # World: owns Terrain + all entity slices
    │   ├── terrain.go      # heightmap grid; Cell struct (elevation, TreeDensity, passability)
    │   ├── objects.go      # PlacedObject: rocks, stumps, decorative items
    │   ├── building.go     # Building struct + spawn timer
    │   ├── lift.go         # Lift struct, queue, rider progress
    │   └── agent.go        # Agent struct + AgentState enum
    │
    ├── sim/
    │   ├── simulation.go   # ticks all agents and buildings each frame; tree collision
    │   └── pathfinder.go   # A* on terrain grid (walking phases only)
    │
    └── save/
        ├── format.go       # JSON-serialisable mirror structs
        └── io.go           # Load / Save scenario to disk
```

---

## Dependencies (`go.mod`)

```
module mountain-mogul

go 1.25.0

require (
    github.com/go-gl/gl                      v0.0.0-20260331235117-4566fea9a276
    github.com/go-gl/glfw/v3.3/glfw          v0.0.0-20260406072232-3ac4aa2bb164
    github.com/go-gl/mathgl                  v1.2.0
    github.com/flopp/go-staticmaps           (indirect — map tile preview)
    github.com/tkrajina/gpxgo                (indirect — geo coordinate parsing)
    github.com/golang/geo                    (indirect — spherical geometry)
    github.com/fogleman/gg                   (indirect — 2D preview rendering)
    golang.org/x/image                       (indirect)
)
```

PNG loading uses stdlib `image/png`. Geo data fetching uses the `geo` package — no third-party API keys required for elevation data.

---

## Key Design Decisions

### Scene Stack
`engine.App` owns the scene stack (push/pop/replace). Only the top scene receives `Update` and `Render`. The `Scene` interface is defined in `engine/scene_iface.go` and re-exported from `scene/scene.go`.

Entities have no `Update()` method. The simulation drives all entity behavior explicitly by ranging over typed slices — no behavior lives on the entity structs themselves.

### Renderer
The renderer is agnostic to simulation types — it only deals in mesh IDs and instance data. Mesh IDs are constants defined in `render/mesh.go`. Static instance buffers only re-upload on scene edits; the dynamic agent buffer re-uploads every frame. Four shader programs share a flat-shading lighting model via `lighting.glsl`, prepended at load time.

`SetBrush(center Vec2, radius float32)` / `ClearBrush()` store brush state on the renderer; the terrain shader reads `uBrushCenter` and `uBrushRadius` to draw the ring analytically in the fragment shader, so it correctly follows the terrain surface.

Camera pan uses `PanDelta(screenDelta)`, which derives right/forward vectors from the camera yaw, so screen-drag direction is always consistent regardless of camera orientation. Mouse-to-terrain picking uses DDA ray-marching (`screenToCell` in `scene/scenario.go`) rather than projecting to a flat plane — required for accuracy on steep terrain.

### Terrain
Two representations: a **simulation grid** (`world.Terrain`, `Cell` structs) for pathfinding and gameplay, and a **render mesh** (heightmap quad grid) for display. Editing terrain updates vertex Y positions in the existing VBO — no full mesh rebuild required.

`Cell.TreeDensity float32` (0–1) replaces per-object tree placement. It drives three systems:
- **Rendering**: `treeCountFromDensity` derives 0–3 tree instances per cell; positions/rotations/scales are stable via a hash.
- **Pathfinding**: `Cell.Walkable()` returns `Passable && TreeDensity < 0.5` — dense forest blocks walking routes.
- **Skiing**: agents take a probabilistic collision roll (`density * dt * treeCollisionRate`) and have their speed reduced (`SkiSpeed * (1 - 0.6 * density)`) in forested cells.

Buildings and lifts still use `Passable bool` as a hard obstacle flag. Rocks, stumps, and other decorative items remain as `PlacedObject` instances (no gameplay effect).

### Geo Import
The `geo` package supports importing real-world terrain into a new scenario. `geocode.go` resolves a search string to a lat/lon bounding box, `elevation.go` fetches heightmap data for that box, `resample.go` fits it to the world grid, and `preview.go` renders a 2D overview for the `terrain_import` scene to display before the player commits.

### Agent State Machine
```
[Building] --spawn--> Walking (A* path to lift base, avoids TreeDensity >= 0.5)
                          |
                     (reach lift base)
                          |
                       Queuing
                          |
                     (front of queue)
                          |
                       Riding (progress 0->1 along lift line)
                          |
                     (progress == 1.0)
                          |
                       Skiing (straight line toward lift base)
                          |  speed = SkiSpeed * (1 - 0.6 * density)
                          |  collision roll per frame in dense cells
                          |
                     (reach lift base)
                          |
                       <loop back to Queuing>
```

Agents loop lift → ski → lift indefinitely for MVP. Skiing is straight-line at speed modulated by tree density — no gravity or friction. Walking follows A* waypoints snapped to terrain elevation.

---

## Rendering Pipeline (per frame)

1. **Clear** — color + depth
2. **Terrain pass** — bind `TerrainShader`, upload `uBrushCenter`/`uBrushRadius` uniforms, draw heightmap mesh; fragment shader draws brush ring analytically using world XZ interpolant `vWorldXZ`
3. **Static pass** — bind `StaticShader`; one `glDrawElementsInstanced` per mesh type (trees, rocks, stumps, buildings, towers); tree instances derived from `Cell.TreeDensity` at batch-rebuild time; buffers only re-upload on scene edits
4. **Cable pass** — bind `StaticShader` (no instancing); draw each lift's procedural cable mesh individually
5. **Dynamic pass** — bind `DynamicShader`; upload full agent instance slice every frame; single instanced draw for all agents; vertex shader derives animation from `gl_InstanceID` + `time` uniform
6. **UI pass** — disable depth test, bind `UIShader` with screen-space ortho; draw menu bar, buttons, font labels

---

## Scene Breakdown

### `StartMenu`
Renders a title + 5 buttons: **Start Game** → push `Scenario` (load `tutorial.json`); **Load Save** → hardcoded save slot; **Scenario Editor** → push `Editor`; **Settings** → stub; **Exit** → close window.

### `Scenario`
Owns `World` and `Simulation`. Menu bar: **Place Building | Place Lift | Glade | Remove**. Active tool governs left-click behavior. Glade tool uses a density brush (radius 2 cells) with a terrain-shader ring preview. Runs `Simulation.Tick(dt)` each frame. Camera: right-click drag or arrow keys to pan, scroll to zoom.

### `ScenarioEditor`
Same world view, simulation paused. Menu bar: **Plant Trees | Glade | Raise Terrain | Lower Terrain | Import Terrain | Save | Back**. Both tree tools show the terrain-shader brush ring preview. Save writes `ScenarioData` JSON to disk.

### `TerrainImport`
Allows importing real-world terrain into a new scenario. Player enters a location name or coordinates → `geo.Geocode` resolves the bounding box → `geo.FetchElevation` downloads the heightmap → `geo.Preview` renders a 2D map tile overlay → player confirms to create a new `World` from the data.

---

## Completed

### T-shaped towers + dual cables
- Lift towers are now procedurally generated T-shapes per lift: a vertical pole (0.7 m wide × 18 m tall) with a horizontal crossbar (5 m wide) perpendicular to the cable direction. Crossbar top aligns exactly with cable height.
- Two cables per lift: **up cable** (+1.5 m lateral offset) and **down/return cable** (−1.5 m), both generated as quad-strip meshes from `generateCableMesh`. Ghost preview during placement shows both cables and towers.
- Tower and cable meshes are stored per-lift (`liftTowerMeshes`, `liftUpCables`, `liftDownCables`) and drawn as world-space meshes using the identity-transform trick (`setCableTransformAttribs`).

### Chair-based lift system
- `Chair` struct: `Progress float32` (0→1 full loop, 0=base, 0.5=top) + `Passengers [2]*Agent`.
- `Lift.ChairPos(progress, terrain)` computes world position + heading for any progress value; used by both simulation and renderer.
- `PlaceLift` auto-spawns chairs at ~30 m intervals around the loop (`ChairSpacingM`).
- `tickLifts` advances all chairs by `lift.Speed / loopLength` fractions/sec. At progress=0.5 (top) passengers are unloaded and start skiing or returning to lodge. At progress=1.0 wrap the chair loads up to 2 skiers from the queue.
- `Lift.Speed` is in **m/s** (default 2.5 m/s — realistic chairlift speed). `Lift.LoopLength()` converts to fractional progress per tick.

### Chair rendering
- Procedural chair mesh (`NewChairMesh`): suspension bar + seat + backrest + footbar, all hanging below the cable-attachment origin. All local Y ≤ 0 so the dynamic-shader limb animation never fires.
- Separate `chairBatch` (dynamic) renders all chairs each frame: grey when empty, blue-tint when carrying passengers.

### Pop-up info windows
- `ui.Window`: floating panel with title bar, close button, label rows (live `getText` callbacks), and stepper rows (+/− buttons for float values).
- Left-clicking a building cell (no tool active) opens a **Lodge** panel showing skier count, spawn rate, and agents currently out.
- Left-clicking within 1 cell of a lift base opens a **Ski Lift** panel showing queue length, on-lift passenger count, chair count, and a speed stepper (0.5–8.0 m/s, step 0.5).
- Popup clicks are consumed before world-click handling so buttons don't accidentally place/remove objects.

### Simulation time scale
- `Simulation.TimeScale` multiplier (default **5×**) compresses real seconds before passing to all sub-ticks. A real 5-minute lift ride takes ~1 minute of wall-clock time at this setting. Easily exposed to UI controls later.

---

## Current Simulation

### Agent Lifecycle

Agents cycle through five states:

```
[Lodge] --Poisson spawn--> Walking (A* path to nearest lift base)
                               |
                          (path complete)
                               |
                            Queuing (waiting at lift base)
                               |
                          (chair arrives, ≤2 per chair)
                               |
                            Riding (position locked to ChairPos)
                               |
                          (progress crosses 0.5 = top)
                               |
                    25% ReturningToLodge    75% Skiing
                         |                      |
                    (reach lodge)          (reach lift base)
                         |                      |
                    SkierCount++           → Queuing
                    agent removed          (loop forever)
```

### Building Spawner

`Building.AdvanceTimer(dt)` implements a Poisson arrival process: inter-arrival times are exponentially distributed with mean `1/MeanSpawnRate`. A spawn is skipped if `SkierCount == 0` (no skiers left in the lodge pool) or no lifts exist. On a failed A* pathfind the agent is immediately removed and the skier returned to the pool.

### Pathfinding

`Pathfinder.FindPath` runs A* with a Manhattan heuristic over the 4-connected terrain grid. A cell is walkable when `Passable && TreeDensity < 0.5`. The destination (lift base) is always reachable even if its cell is structurally blocked.

### Lift Tick

Every chair advances by `lift.Speed / lift.LoopLength()` fractional progress per sim-second. Crossing `progress = 0.5` unloads passengers at the lift top; wrapping past `progress = 1.0` loads up to 2 skiers from the head of the queue.

### Skiing Physics

Both `StateSkiing` and `StateReturningToLodge` use the same slope-physics model:

| Parameter | Value | Meaning |
|---|---|---|
| `g` | 9.81 m/s² | gravitational acceleration |
| `μ` | 0.05 | kinetic friction (groomed snow) |
| `kDrag` | 0.01 m⁻¹ | air resistance per unit mass |

Net acceleration per frame: `a = g·sinθ − μ·g·cosθ − kDrag·v²`

`sinθ` is derived from the terrain normal at the agent's current XZ position (`cosθ = normal.y`). Speed is integrated forward (`v += a·dt`, floored at 0) and the agent moves at speed `v` in a straight line toward its target (lift base or lodge).

### Known Weaknesses

- **No trail network.** Skiers head straight-line toward the lift base — they will happily ski uphill, cross impassable terrain, or clip through trees.
- **Single-lift targeting.** At spawn, the agent is assigned the nearest lift and never re-evaluates. No multi-lift resort routing.
- **No grooming effect.** The `Cell.Groomed` flag exists but is unused; μ is always 0.05 regardless.
- **No tree-density speed penalty during skiing.** `Cell.TreeDensity` slows agents only in the original design note; the physics tick ignores it.
- **Straight-line return.** `StateReturningToLodge` skis in a straight line to the lodge; no path following or terrain avoidance.
- **No agent cap.** Buildings spawn indefinitely from `SkierCount`; there is no global or per-lift agent limit.

---

## Next Steps
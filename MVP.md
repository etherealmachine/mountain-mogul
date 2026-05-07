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
    │   ├── button.go       # rectangular clickable element with label
    │   ├── window.go       # popup info panels (lodge / lift detail, escape menu)
    │   ├── slider.go       # vertical slider used by the editor brush-radius control
    │   └── textinput.go    # single-line text input used by the Save-As prompt
    │
    ├── scene/
    │   ├── scene.go        # Scene interface (also defined in engine/scene_iface.go)
    │   ├── startmenu.go    # Continue / New Game / Load Game / Editor / Testbeds / Settings / Exit
    │   ├── scenariopicker.go  # picker for the asset scenarios (drives "New Game")
    │   ├── savelist.go     # picker for named user saves (drives "Load Game")
    │   ├── testbedmenu.go  # picker for the testbed registry
    │   ├── escapemenu.go   # in-game pause overlay (Resume / Save / Load / Main Menu)
    │   ├── scenario.go     # main gameplay scene; DDA screenToCell; Save-As prompt
    │   ├── editor.go       # scenario editor scene; brush-radius slider
    │   └── terrain_import.go  # real-world terrain import flow (geocode -> preview -> confirm)
    │
    ├── world/
    │   ├── world.go        # World: owns Terrain + all entity slices; ID counter
    │   ├── terrain.go      # heightmap grid; Cell struct (elevation, TreeDensity, passability)
    │   ├── objects.go      # PlacedObject: rocks, stumps, decorative items
    │   ├── building.go     # Building struct + Poisson spawn timer
    │   ├── lift.go         # Lift struct (chairs, queue, ChairPos)
    │   └── agent.go        # Agent struct (implicit-state fields) + Activity helper
    │
    ├── ai/
    │   └── types.go        # persistent skier-AI types: SkillLevel, TechniqueSet,
    │                       # SkierTraits, Route, MotorState (lives outside sim/world
    │                       # to break the import cycle)
    │
    ├── sim/
    │   ├── simulation.go   # top-level Tick: buildings, lifts, agent dispatch
    │   ├── skiing.go       # 5-layer skier-AI pipeline (Route / Perception / Steering /
    │   │                   # Motor / Physics) + Balance/Fallen mechanic + debug overlay
    │   ├── pathfinder.go   # A* on terrain grid (walking phases only)
    │   ├── testbeds.go     # Testbed registry + fluent builder DSL (scene().slope()...)
    │   ├── headless.go     # `-testbed <prefix>` CLI runner: no GLFW, writes trace + summary
    │   └── recorder.go     # Recorder interface + CSVRecorder for per-tick AI trace logs
    │
    └── save/
        ├── format.go       # JSON-serialisable mirror structs (entity IDs round-trip)
        └── io.go           # SavesDir / ListSaves / SaveAs / Load (named user saves)
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
- **Skiing**: the AI's perception layer probes `TreeDensityAt` along a forward cone; the steering layer bends the axis heading away from hazards and slows the target speed in dense centre cells; the stress model drains balance near close hazards. There is no separate collision roll or speed multiplier — trees affect skiers through what they perceive and how they react.

Buildings and lifts still use `Passable bool` as a hard obstacle flag. Rocks, stumps, and other decorative items remain as `PlacedObject` instances (no gameplay effect).

### Geo Import
The `geo` package supports importing real-world terrain into a new scenario. `geocode.go` resolves a search string to a lat/lon bounding box, `elevation.go` fetches heightmap data for that box, `resample.go` fits it to the world grid, and `preview.go` renders a 2D overview for the `terrain_import` scene to display before the player commits.

### Agent State (Implicit)

There is no `AgentState` enum. An agent's situation is implicit in the
combination of its fields:

| Field | When set | Meaning |
|---|---|---|
| `Fallen` + `FallTimer` | Balance hit zero | Frozen for ~4 s; resumes toward the same TargetID |
| `OnLiftID` | Boarded a chair | Position is locked to `Lift.ChairPos`; locomotion suspended |
| `Queued` | Walked into the lift queue | Waiting in `lift.Queue`; chair-load drains the queue |
| `Path` non-empty + `PathIdx < len(Path)` | A* path was assigned at spawn | Walking the path at `WalkSpeed` |
| `TargetID == 0` | Just unloaded with no goal | Idle (rare; quickly re-targeted) |
| (otherwise, `TargetID != 0`) | Default | Locomoting toward the resolved target |

`world.Activity(world, agent)` returns a single human-readable label
("Walking", "On Lift", "To Lift", "To Lodge", "Fallen"…) by ranking the
checks above. Used by the follow HUD, debug overlays, the CSV recorder, and
the headless-trace event lines.

The simulation's per-agent dispatch (`tickAgents`) uses the same priority:
`Fallen → OnLiftID → Queued → Path → tickLocomote`. `tickLocomote` itself
calls `shouldSki` to choose between the skiing pipeline and a straight-line
walk based purely on whether the goal lies in the downhill direction; slope
magnitude doesn't gate the choice (the ski physics handle gentle terrain
naturally — friction dominates on flats).

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
Renders a title + buttons: **Continue** (only when ≥1 save exists; loads the
newest), **New Game** → push `ScenarioPicker`, **Load Game** → push
`SaveList`, **Scenario Editor** → push `Editor`, **Testbeds** → push
`TestbedMenu`, **Settings** (stub), **Exit**. The Continue button appears
and disappears in response to save state without an explicit lifecycle hook
(re-evaluated each frame).

### `ScenarioPicker`
Lists the asset scenarios under `assets/scenarios/`. Selecting one creates
a `Scenario` via `NewScenarioFromFile`.

### `SaveList`
Lists the user's named saves from `~/.mountain-mogul/saves`, newest first.
Selecting one calls `NewScenarioFromFile` against the save path; the
Scenario remembers the basename so subsequent Save clicks default to
overwriting the same slot.

### `TestbedMenu`
Lists the entries in `sim.Testbeds`. Selecting one calls
`NewScenarioFromTestbed`, which builds the world and remembers the testbed
builder so the menu bar can show a **New Seed** button.

### `Scenario`
Owns `World` and `Simulation`. Menu bar: **Place Building | Place Lift |
Glade | Remove** (+ **New Seed** in testbed mode). Active tool governs
left-click behavior. Glade tool uses a density brush with a terrain-shader
ring preview. Runs `Simulation.Tick(dt)` each frame.

Save / Load (via the escape menu) are gated by `saveAllowed`: enabled for
asset scenarios and named saves, disabled for testbeds. Save opens a modal
**Save As** prompt (`ui.TextInput`) pre-filled with the last name used (or
a fresh `save-YYYY-MM-DD-HHMM` timestamp on first save). Load pops back to
the start menu and pushes `SaveList` so the picker UX is uniform.

Camera: right-click drag or arrow keys to pan, scroll to zoom.

### `Editor`
Same world view, simulation paused. Menu bar: **Plant Trees | Glade |
Raise Terrain | Lower Terrain | Import Terrain | Save | Back**. Plant /
Glade tools show a brush-radius **vertical slider** (`ui.VSlider`) on the
left edge — the brush ring preview tracks the slider value live. Importing
terrain replaces the entire world (buildings, lifts, agents, trees) so old
placements don't dangle on the new mountain.

### `TerrainImport`
Allows importing real-world terrain into a new scenario. Player enters a
location name or coordinates → `geo.Geocode` resolves the bounding box →
`geo.FetchElevation` downloads the heightmap → `geo.Preview` renders a 2D
map tile overlay → player confirms to create a new `World` from the data.

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
- `Simulation.TimeScale` multiplier (default **5×**) compresses real seconds before passing to all sub-ticks. A real 5-minute lift ride takes ~1 minute of wall-clock time at this setting. Speed/pause buttons in the menu bar expose 1× / 5× / 10× and a pause toggle.

### Skier-AI pipeline + testbeds + headless mode
- Five-layer skier AI (Route / Perception / Steering / Motor / Physics) with
  technique dispatch (straight, pizza, wedge-turn, parallel, hockey,
  sideslip) gated by skill-level `TechniqueSet`. Persistent AI state
  (`SkierTraits`, `Route`, `MotorState`, `Balance`) lives in `internal/ai`.
- Implicit-state agents — no `AgentState` enum. The `world.Activity` helper
  derives a label from `Fallen` / `OnLiftID` / `Queued` / `Path` /
  `TargetID` for HUDs, debug overlays, recorder, and headless trace.
- `Testbed` registry + fluent builder DSL (`scene().slope().lodge().skier()
  ...`) drives both an in-game `TestbedMenu` (with a **New Seed** button)
  and a `-testbed "<name prefix>"` CLI runner.
- CSV recorder (`sim.Recorder` interface, `sim.CSVRecorder`) logs one row
  per tick of the followed agent — every perception/intent/motor/balance
  signal — for offline analysis.
- Falls: `Balance ∈ [0, 1]` drains under stress (over-comfort speed/slope,
  near hazards, technique cost). At zero the agent enters a `Fallen` window
  for ~4 s, then resumes toward the same TargetID.

### Named saves + start-menu UX
- Saves are named files under `~/.mountain-mogul/saves/`. The Scenario
  scene's escape-menu Save opens a modal **Save As** prompt
  (`ui.TextInput`); Load pops to the start menu and pushes `SaveList`.
- Start menu: **Continue** (newest save), **New Game** (`ScenarioPicker`),
  **Load Game** (`SaveList`), **Scenario Editor**, **Testbeds**, Settings,
  Exit. Continue auto-shows/hides as save state changes.
- Save format preserves building / lift / agent IDs so chair-passenger
  and queue references survive a round trip — skiers mid-ride re-mount
  the same chair on load.

### Editor brush-radius slider
- `ui.VSlider` on the editor's left edge controls plant/glade brush radius
  live. The terrain-shader brush ring preview tracks the slider value.
- Importing terrain now wipes the world (buildings, lifts, agents, trees)
  rather than overlaying onto the old layout.

---

## Current Simulation

### Agent Lifecycle

```
[Lodge] --Poisson spawn--> Walking  (A* path to nearest lift base)
                              │
                         (path complete)
                              │
                           Queuing
                              │
                         (chair arrives, ≤2 pax)
                              │
                           Riding   (position = lift.ChairPos)
                              │
                         (progress crosses 0.5 — top)
                              │
                  25% target lodge    75% target lift base
                         │                    │
                  (re-skier pipeline)    (re-skier pipeline)
                         │                    │
                    arrive lodge         arrive lift base
                    SkierCount++         → Queuing
                    agent removed        (loop forever)
```

`Simulation.Tick(dt)` advances buildings, lifts, then agents. Per-agent
dispatch picks one of: `tickFallen`, `tickRiding`, no-op (Queued),
`tickPath`, or `tickLocomote`. `tickLocomote` calls `shouldSki(terrain,
pos, target)` to fork between the skiing pipeline and a straight walk.

### Building Spawner

`Building.AdvanceTimer(dt)` implements a Poisson arrival process:
inter-arrival times are exponentially distributed with mean
`1/MeanSpawnRate`. A spawn is skipped if `SkierCount == 0` or no lifts
exist. On a failed A* pathfind the agent is removed and the skier returned
to the pool. New agents draw a `SkillLevel` from the lodge distribution
(60/30/10 beginner/intermediate/advanced) and `Traits = TraitsFor(skill)`.

### Pathfinding

`Pathfinder.FindPath` runs A* with a Manhattan heuristic over the
4-connected terrain grid. A cell is walkable when `Passable && TreeDensity
< 0.5`. The destination (lift base) is always reachable even if its cell
is structurally blocked.

### Lift Tick

Every chair advances by `lift.Speed / lift.LoopLength()` fractional
progress per sim-second. Crossing `progress = 0.5` unloads passengers,
restores their balance if it was low, snaps their position to the lift top,
and reorients their heading toward a fresh top-target. Wrapping past
`progress = 1.0` loads up to 2 skiers from the head of `lift.Queue`.

`pickTopTarget` chooses where the agent goes after unloading: with
probability `lodgeReturnProb` (0.25) a randomly-picked lodge becomes the
new TargetID, otherwise it's the lift's own base. Both are then driven by
the same skier pipeline.

### Skier AI Pipeline (`internal/sim/skiing.go`)

Each tick of a skiing agent runs five thin layers, each a near-pure
function with a small typed input/output so layers are testable in
isolation. Persistent per-agent state lives on the Agent (`Traits`,
`Route`, `Motor`, `Balance`); per-tick types (`Perception`, `Intent`,
`MotorCmd`, `Hazard`) are sim-internal and never stored.

| # | Layer | Reads | Writes |
|---|---|---|---|
| 1 | **Route** | TargetID, sim-time | `agent.Route` (slow plan; refreshed every 2 s) |
| 2 | **Perception** | terrain normal at agent, slope ahead, axis to goal, 5-probe tree cone | `Perception` value |
| 3 | **Steering** | Traits + Perception | `Intent`: axis heading (seek + fall-line bias, attenuated by slope), desired speed, urgency |
| 4 | **Motor** | Traits + prev MotorState + Intent + Perception | `MotorCmd` (heading, scrub, balance cost, max turn rate) + new MotorState |
| 5 | **Physics** | MotorCmd + Perception + dt | integrates heading, speed, position |

**Techniques.** The motor layer dispatches to one of six techniques bounded
by the skier's `TechniqueSet` (`KitFor(SkillLevel)`):

- `TechStraight` — schuss; no scrub
- `TechPizza` — wedge; constant scrub, beginner default
- `TechWedgeTurn` — direction change while in the wedge
- `TechParallel` — linked S-turns; arc width anchored on slope-vs-comfort,
  with a minimum dwell per phase (`parallelMinDwell`) to stop sub-second
  pinging across the fall line
- `TechHockey` — hard 90° edge-set pulse (`hockeyDurationS = 0.6 s`),
  triggered when intent.Urgency > 0.8; advanced only
- `TechSideslip` — perpendicular descent on steeps; intermediate +

Beginners get straight + pizza + wedge-turn; intermediates add parallel +
sideslip; advanced add hockey.

**Physics.** `applyMotor` integrates with two friction terms:

- `muBase = 0.04` — base kinetic friction
- `muEdge = 0.20` — carving friction, scaled by the cross-fall component of
  heading vs fall-line (so edged turns scrub speed even at low speed)
- `kDrag = 0.01` — air drag

```
a = g·sinθ·cos(headingOff)
   − muBase·g·cosθ
   − muEdge·g·cosθ·|sin(headingOff)|
   − kDrag·v²
   − cmd.Scrub
```

`headingOff` is the angle between heading and the fall line. Heading
rotates toward `cmd.Heading` at `min(maxAngularSpeed, cmd.MaxTurnRate)`.
There's a `skiWalkSpeed = 2 m/s` floor representing skating/poling.

**Balance & falls.** `stressDelta` returns a per-second drain rate for
`Balance ∈ [0, 1]`: base recovery +0.15/s, drained by speed over comfort,
slope-ahead over comfort, near tree hazards, and the active technique's
`BalanceCost`. When Balance reaches 0 the agent enters `Fallen` for 4
sim-seconds; on recovery Balance is reset to 0.7 and the same TargetID is
resumed.

**Tree perception.** `scanTrees` samples `TreeDensityAt` at five forward
probes (centre + ±½·probeAngle + ±probeAngle) and returns hazards above a
noise threshold. The steering layer uses the worst hazard to bend the axis
heading away (cross-product side test) and scales target speed down when
the centre probe is in a dense cell. Trees thus affect skiing through
perception/steering/balance — there's no longer a separate
"speed-multiplier-by-cell-density" hack.

### `shouldSki` Dispatch

`shouldSki(terrain, pos, target)` returns true iff the goal is in the
downhill direction of the local fall line. Slope magnitude is irrelevant:
gentle and flat sections are handled by the ski physics naturally
(low gravity accel, friction dominates, `skiWalkSpeed` floor keeps motion
forward). Flat or uphill goals fall back to `tickWalkToward`. This
replaces the old `skiSlopeThreshold` of 5°.

### Known Weaknesses

- **No trail network.** Skiing skiers obey the fall line + tree
  avoidance, but there are no signs, groomed-trail bias, or trail-network
  pathfinding — so a skier can still side-step into terrain the designer
  didn't intend.
- **Single-lift targeting at spawn.** The agent picks the nearest lift at
  spawn and never re-evaluates. No multi-lift resort routing.
- **No grooming effect.** `Cell.Groomed` exists but `muBase` is constant.
- **Lodge return uses the same skier pipeline.** Better than the old
  straight-line return, but the lodge can be uphill from the lift top and
  the dispatch falls back to walking — no "ski to the trail bottom and
  walk in" logic yet.
- **No agent cap.** Buildings spawn indefinitely from `SkierCount`; there
  is no global or per-lift limit.

---

## Testbeds & Headless Iteration

The skier AI is iterated on against a registry of small, deterministic
scenarios. Each `Testbed` builds a fresh `*world.World` from a seeded
`*rand.Rand`, so the visual mode and the headless runner see byte-identical
worlds for the same seed.

### Registry

`sim.Testbeds` is a slice of:

```go
type Testbed struct {
    Name  string
    Build func(rng *rand.Rand) *world.World
    Seed  int64
}
```

Names are human-readable and used as match prefixes by the CLI, so they
should start with the distinguishing detail (e.g. `"10 degree slope,
intermediate skier"` lets a user write `-testbed "10 degree slope"`).
`FindTestbed(prefix)` does a case-insensitive prefix match and errors on
zero or multiple matches.

### Fluent Builder DSL

Testbed builders chain a small DSL so a scene reads like a sentence:

```go
scene(40, 60).slope(15).lodge().skier(ai.SkillIntermediate).build()
scene(40, 60).slope(15).lodge().skier(ai.SkillIntermediate).
             treePatch(20, 30, 6, 0.8).build()
scene(40, 80).runout(50, 18, 3).lodge().skier(ai.SkillAdvanced).build()
```

Methods are split into terrain shaping (`flat`, `slope`, `runout`),
target placement (`lodge`, `lodgeAt`), skier spawn (`skier`, `skierAt` —
auto-targets the most recently placed lodge), and obstacles (`treePatch`).
`build()` returns the finished world. Lodges have spawning disabled — they
are nav targets, not sources.

### In-Game Picker (`TestbedMenu`)

The start-menu **Testbeds** button pushes `TestbedMenu`, which lists every
entry in the registry. Selecting one calls `NewScenarioFromTestbed`, which
builds the world and stores the builder closure so the in-scene menu bar
can offer a **New Seed** button — re-rolls the run with a fresh RNG seed
without leaving the scene. Save / Load are disabled in testbed mode
(`saveAllowed = false`).

### Headless Runner (`-testbed`)

```
go run . -testbed "<name prefix>" [-seed N] [-sim-seconds 240] [-sample 0.5]
go run . -testbed list   # print all testbed names
```

`sim.RunHeadless` builds the testbed world, wires in a `traceRecorder`
(text output), and ticks at a fixed `dt = 1/60` until either the agent
arrives or `sim-seconds` is exhausted. It writes:

1. A header line (testbed name, seed, sim-seconds cap, agent skill,
   start position, target, distance).
2. Activity transition lines (`! t=12.34  Walking → Queuing`) flushed as
   they happen.
3. A periodic tabular trace, one row every `sample` sim-seconds:
   `t, activity, pos, heading, speed, technique, urgency, balance,
   fall_line, axis_head, slope_cos, probes…`.
4. An arrival line (`arrived at t=87.42s`) or a final-position summary
   on timeout.

The headless runner is the primary tool for reasoning about agent
behaviour: rather than asserting against thresholds it dumps what the AI
perceived and decided each tick, so a regression shows up as a different
choice in a readable column.

### CSV Recorder

`sim.Recorder` is the Recorder-pattern interface the simulation calls once
per skiing agent per tick. The headless runner uses a text-mode
implementation; the in-game scene wires `sim.CSVRecorder` instead, writing
a CSV row per tick of the followed agent (`F-key` follow). The CSV header
covers every signal that drives the AI tick — perception inputs, intent,
motor command, balance — so a logged run can be reasoned about with a
spreadsheet or a quick Python plot. Recording is opt-in (default `nil`)
and skipped on the hot path when no recorder is attached.

---

## Save System

User saves live in `~/.mountain-mogul/saves/<name>.json` (or
`./mountain-mogul-saves/` when the home dir is unknown). The save format
preserves every entity's `ID` so cross-references survive a round trip:

- **Buildings** keep their ID so `agent.TargetID` references resolve.
- **Lifts** save chair `Progress` and chair-passenger IDs, plus the
  ordered queue IDs. Skiers mid-ride re-mount the same chair on load
  rather than freezing in mid-air.
- **Agents** keep their ID so the lift-passenger / queue back-references
  rebind on load.

`World.SetMinNextID(maxRestoredID)` bumps the world's ID counter past
everything restored so future spawns don't collide. Old saves without `id`
fields fall back to fresh IDs (legacy compatibility).

---

## Next Steps
# Plan: Player-Defined Trails

## Context

Currently skiers descend via `SkiToLift / SkiToLodge / SkiToParking` — purely elevation-gated
(any lift top can reach any lift base 20 m lower, free-roaming). This feature makes trails
first-class entities: players paint cell areas to define named, difficulty-rated runs, and the
game builds a connectivity graph from those areas. Skiers prefer defined trails but can still
free-roam (off-trail is penalized by skill level, not forbidden).

GUESTS.md §"Trails as first-class entities" and §"New actions" describe the intent; this plan
revises the data model to be area-based with derived connectivity.

---

## Core insight: trails are areas, not routes

A trail is defined entirely by the **cells it covers**. The from/to connectivity the planner
needs is **derived** from which entity footprints overlap those cells — lift tops, lift bases,
lodges, parking lots, and other trails. Players paint cells; the game builds the graph.
Junctions between trails are implicit: two trails that share or touch cells are connected at
that shared area, with no special junction entity needed.

---

## Data model

**New file: `internal/world/trail.go`**

```go
type Trail struct {
    ID         uint64
    Name       string
    Difficulty TerrainDifficulty   // DiffGreen | DiffBlue | DiffBlack
    Cells      [][2]int            // grid cells belonging to this trail
}
```

No explicit `FromEntityID` / `ToEntityID` — connectivity is derived (see TrailGraph below).

Add to **`internal/world/world.go`**:
```go
Trails []*Trail
```

Helpers in `trail.go`:
- `w.PlaceTrail(name string, diff TerrainDifficulty) *Trail` — creates empty trail; cells painted interactively
- `w.DeleteTrail(id uint64)`
- `w.AddTrailCells(id uint64, cells [][2]int)` / `RemoveTrailCells` — incremental edits
- Auto-naming: `nextTrailDefaultName` counter → "Run 1", "Run 2", …

---

## Trail graph (derived, cached)

**New file: `internal/world/trail_graph.go`**

```go
type TrailEdge struct {
    TrailID    uint64
    FromEntity uint64   // lift ID (at its top), building ID, or trail ID (junction entry)
    ToEntity   uint64   // lift ID (at its base), building ID, or trail ID (junction exit)
    Distance   float32  // straight-line between entity positions (planning approximation)
}

type TrailGraph struct {
    Edges []TrailEdge
}

func BuildTrailGraph(w *World) *TrailGraph
```

**Graph building logic** (called on trail add/remove/edit, result cached on `World`):

1. For each trail, collect all entities whose footprint overlaps the trail's cells:
   - Lift tops and bases (check if lift base/top cell is in trail cells)
   - Building positions (lodge, parking — check if building's cell is in trail cells)
   - Other trails (check for shared or adjacent cells → trail-to-trail edges)

2. For each pair of collected entities (A, B) where B is lower elevation than A:
   add `TrailEdge{trail.ID, A.ID, B.ID, dist(A.pos, B.pos)}`

3. For trail-to-trail: when Trail T1 cells overlap Trail T2 cells, and T2 has downhill
   entities not in T1's entity set, add edges from T1's uphill entities through the junction
   to T2 (represented as `ToEntity = T2.ID` with `ToKind` distinguishing trail IDs).

The graph is rebuilt by `world.RebuildTrailGraph(w)` and stored as `w.TrailGraph *TrailGraph`.
`RebuildTrailGraph` is called from:
- `w.PlaceTrail` / `w.DeleteTrail`
- `w.AddTrailCells` / `RemoveTrailCells`
- `scenario.installWorld` after load

---

## Save / load

**`internal/save/format.go`**

```go
type TrailData struct {
    ID         uint64    `json:"id,omitempty"`
    Name       string    `json:"name,omitempty"`
    Difficulty uint8     `json:"diff,omitempty"`
    Cells      [][2]int  `json:"cells,omitempty"`
}
```

Add `Trails []TrailData` to `ScenarioData`. The `TrailGraph` is NOT saved — it's recomputed
on load. Standard `worldToData` / `dataToWorld` 1:1 conversion.

---

## GOAP planner

**New file: `internal/ai/goap/ski_trail.go`**

```go
type SkiTrail struct {
    TrailID    uint64
    FromEntity uint64   // matches the guest's current anchor (AtLiftTop, AtLodge, AtTrailEnd…)
    ToEntity   uint64   // where this trail edge leads
    ToKind     EdgeKind // KindLiftBase, KindBuilding, KindTrail
    Distance   float32
}

func (a *SkiTrail) Precondition(s *WorldSnapshot, w *world.World) bool {
    // Guest must be at the FromEntity anchor
    switch resolveEntityKind(w, a.FromEntity) {
    case KindLiftTop:   return s.AtLiftTop == a.FromEntity
    case KindTrail:     return s.AtTrailEnd == a.FromEntity
    case KindBuilding:  return s.AtLodge == a.FromEntity || s.AtParking == a.FromEntity
    }
}

func (a *SkiTrail) Apply(s *WorldSnapshot, w *world.World) {
    s.AtLiftTop = 0
    s.AtTrailEnd = a.TrailID   // so connecting trails can fire next
    switch a.ToKind {
    case KindLiftBase:  s.AtLiftBase = a.ToEntity
    case KindBuilding:  // set AtLodge or AtParking based on building type
    case KindTrail:     // AtTrailEnd already set; connecting trail's Precondition uses it
    }
}

func (a *SkiTrail) Cost(s *WorldSnapshot, w *world.World) float32 {
    return a.Distance / skiSpeedMps
}
```

**`internal/ai/goap/action.go` — `ApplicableActions`**

Replace the `s.AtLiftTop != 0` ski-down block with graph-driven generation:

```go
// Trail-based descents (from any anchor type)
currentAnchor := currentAnchorID(s)  // whichever At* field is set
if currentAnchor != 0 {
    for _, edge := range w.TrailGraph.EdgesFrom(currentAnchor) {
        a := &SkiTrail{
            TrailID: edge.TrailID, FromEntity: edge.FromEntity,
            ToEntity: edge.ToEntity, ToKind: edge.ToKind, Distance: edge.Distance,
        }
        if a.Precondition(s, w) {
            out = append(out, a)
        }
    }
}

// Free-roam fallback (always generated; skill-based cost penalty makes it less attractive)
if s.AtLiftTop != 0 {
    for _, l := range w.Lifts { /* SkiToLift with offTrailPenalty */ }
    for _, b := range w.Buildings { /* SkiToLodge / SkiToParking with offTrailPenalty */ }
}
```

**Off-trail penalty** added to `SkiToLift / SkiToLodge / SkiToParking Cost()`:

| Skill | Penalty |
|---|---|
| Beginner | +120 s |
| Intermediate | +45 s |
| Advanced | +10 s |
| Pro | 0 s |

**`internal/ai/types.go`**: add `ActSkiTrail`, `TrailID uint64`, `FromEntity / ToEntity uint64`
to `PlanAction`.

**`internal/ai/goap/state.go`**: add `AtTrailEnd uint64` to `WorldSnapshot`; include in
`stateKey()`. Clear `AtTrailEnd` in `JoinQueue.Apply`, `WalkToLift.Apply`, `RideLift.Apply`.

**`internal/sim/simulation.go` — `onPlanStepStart`**

New case `ai.ActSkiTrail`:
- Resolve `ToEntity` to a world position
- If ToKind = LiftBase: `a.Plan.Target = lift.BackOfQueueWorldPos(...)`, `a.Plan.Goal = GoalLift`
- If ToKind = Building (parking): `a.Plan.Target = parkingWorldPos(...)`, `a.Plan.Goal = GoalDepart`
- If ToKind = Building (lodge): `a.Plan.Target = buildingWorldPos(...)`, `a.Plan.Goal = GoalNone`
- If ToKind = Trail: `a.Plan.Target` = centroid of shared cells between this trail and the target trail

Also update `ToPlanActions` to emit `ai.PlanAction` with trail fields.

---

## Trail painting UI

**`internal/scene/scenario.go`**

Single tool mode: `toolTrailPaint`. Operates on the "active trail" (the trail currently being
edited).

State held on `Scenario`:
```go
activeTrailID    uint64
trailDifficulty  world.TerrainDifficulty  // Green/Blue/Black — cycled by toolbar click
trailPainting    bool
```

**Flow:**
1. Player clicks Trail tool in toolbar → cycles `trailDifficulty` if already active, else
   creates a new trail (`w.PlaceTrail(name, diff)`) and enters `toolTrailPaint`
2. Left-drag: paint cells onto `activeTrailID` (`w.AddTrailCells`), rebuild `TrailGraph`
   each paint stroke end
3. Right-drag: erase cells from `activeTrailID`
4. Click another existing trail centroid label: sets `activeTrailID` to that trail (for editing)
5. Escape or click different tool: deactivates paint mode, commits trail

**Brush**: fixed radius in cells (e.g., 2-cell radius ≈ 10 m), same as terrain brushes.
Hover preview: highlight the brush footprint cells.

---

## Trail rendering (two-layer)

### Layer 1: Trail area — overlay (toggleable)

Shown when the "Trails" overlay bit is set (new entry in the overlay panel).

Color each trail's cells on the terrain using the existing overlay rendering path — same
mechanism as slope/grooming overlays. Each cell in a trail is tinted by the trail's
difficulty color with alpha ~0.4:
- Green: `#55AA55`
- Blue: `#4488CC`  
- Black: `#333333`

Where two trails overlap (junction): blend colors or show the higher-difficulty color.

`BuildTrailOverlay(w)` generates the per-cell color array; updated on trail change.

### Layer 2: Trail name labels — default-on, toggleable

Trail name rendered at the centroid of the trail's cell bounding box. Shown by default;
toggled via a "Trail names" toggle in the overlay panel. Difficulty-colored text.

`RenderTrailLabels(w)` iterates `w.Trails`, computes centroid of `trail.Cells`, draws
`trail.Name` at that world position using the existing `Font` renderer.

---

## Trail info popup (inspect / rename / delete)

Left-click near a trail name label (or on a painted trail cell in overlay mode) opens a
`ui.Window` popup:
- Editable `TextInput` for name
- Difficulty selector (Green / Blue / Black buttons)
- Delete button → `w.DeleteTrail(id)`, rebuild `TrailGraph`, replan affected guests

**Replan on deletion:** scan `w.OnMountain` for guests whose current plan step has
`TrailID == deletedID`; clear their `Plan.Steps` so `tickPlanning` replans next tick.

---

## `Lift.Services` — derived

`ServicesForLift(w, liftID)` ORs the `Difficulty` of all trails with edges starting at that
lift's top. Read by the lift popup UI and the GOAP `Explore` goal. No manual setter.

---

## Files modified

| File | Change |
|---|---|
| `internal/world/trail.go` | **new** — Trail struct, PlaceTrail/DeleteTrail, cell helpers |
| `internal/world/trail_graph.go` | **new** — TrailGraph, BuildTrailGraph, EdgesFrom |
| `internal/world/world.go` | add `Trails []*Trail`, `TrailGraph *TrailGraph`; remove manual `Lift.Services` |
| `internal/save/format.go` | add `TrailData`, `ScenarioData.Trails`, worldToData/dataToWorld |
| `internal/ai/goap/ski_trail.go` | **new** — SkiTrail action |
| `internal/ai/goap/action.go` | `ApplicableActions`: graph-driven SkiTrail generation + off-trail penalty |
| `internal/ai/goap/state.go` | add `AtTrailEnd` to `WorldSnapshot` + `stateKey()`; clear in lift/walk actions |
| `internal/ai/types.go` | `ActSkiTrail` kind, `TrailID/FromEntity/ToEntity` on `PlanAction` |
| `internal/sim/simulation.go` | `onPlanStepStart` + `ToPlanActions` for `ActSkiTrail` |
| `internal/scene/scenario.go` | trail paint tool (toolTrailPaint), brush hover, click popup |
| `internal/render/renderer.go` | `BuildTrailOverlay`, `RenderTrailLabels`; cell color overlay + labels |
| `GUESTS.md` | move Trail from Future Extensions to implemented |
| `TRAILS.md` | **new** — player-facing trail system doc |

---

## Verification

1. `go build ./...` + `go vet ./...` — no errors
2. Existing testbed (no trails) → skiers navigate normally; off-trail penalty is a no-op
   when no trails exist (no edges in TrailGraph, so penalty only applies when trails do exist)
3. Paint a green trail from lift A top cells to lift B base cells → `TrailGraph` gets edge
   (liftATop → liftBBase); F4 debug panel shows `SkiTrail(#N)` in a Beginner's plan
4. Paint a connecting trail from the same mid-cells down to parking → F4 shows both
   SkiTrail options from lift A top; trail-to-trail junction works
5. Pro guest at lift A → F4 shows SkiTrail AND SkiToLift (free-roam) at comparable cost
6. Enable Trails overlay → cells colored green/blue/black on terrain
7. Toggle trail names → labels appear/disappear
8. Click trail label → popup; rename and difficulty change work; delete replans guests
9. Save + reload → trail cells preserved, TrailGraph rebuilt correctly on load

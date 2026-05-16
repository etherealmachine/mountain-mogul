# Trails

## Core concept: trails are areas, not routes

A trail is defined entirely by the **cells it covers**. Connectivity is **derived** — the game
inspects which entity footprints (lift tops/bases, lodges, parking, other trails) overlap a
trail's cells and builds a directed graph from those overlaps. Players paint cells; the game
infers the graph. No explicit from/to authoring needed.

---

## Data model

**`internal/world/trail.go`**

```go
type Trail struct {
    ID         uint64
    Name       string
    Difficulty TerrainDifficulty   // DiffGreen | DiffBlue | DiffBlack (bitfield)
    Cells      [][2]int            // grid cells [x, z] belonging to this trail
}
```

`Trail.Centroid()` returns the average world-space XZ of the cell set.

`TerrainDifficulty` is defined in `internal/world/lift.go` as a 3-bit bitfield. It is currently
pure metadata — the AI doesn't gate on it. `ServicesForLift(liftID)` ORs the difficulty of all
trails whose edges depart from a lift's top, and falls back to all three if no trails are
attached (prevents guests locking out of lifts).

`World.Trails []*Trail` is the authoritative list; `World.TrailGraph *TrailGraph` is the derived
cache.

---

## Trail graph

**`internal/world/trail_graph.go`**

```go
type TrailGraph struct {
    Edges    []TrailEdge
    byFromID map[uint64][]int   // index into Edges keyed by FromID
}

type TrailEdge struct {
    TrailID  uint64
    FromID   uint64
    FromKind EdgeKind    // KindLiftTop | KindTrail | KindBuilding
    ToID     uint64
    ToKind   EdgeKind    // KindLiftBase | KindTrail | KindBuilding
    Distance float32     // straight-line metres
}
```

`BuildTrailGraph(w *World)` rebuilds the whole graph:

1. For each trail, collect entity footprints overlapping its cells (lift tops/bases, buildings,
   other trails — via `trailsTouching`).
2. For each pair where the source is higher than the destination by at least
   `trailMinDescentMeters = 20 m`, emit a `TrailEdge`.
3. Also emit edges where the source is a trail junction (`FromKind = KindTrail`), so trail
   chaining works.

`EdgesFrom(fromID uint64) []TrailEdge` — O(1) lookup via `byFromID`.

`RebuildTrailGraph()` is called by `PlaceTrail`, `DeleteTrail`, `AddTrailCells`,
`RemoveTrailCells`, and `dataToWorld` after load.

---

## GOAP integration

**`internal/ai/goap/ski_trail.go`**

`SkiTrail` is the GOAP action for a single trail traversal:

- **Precondition:** guest's current anchor (`AtLiftTop`, `AtTrailEnd`, `AtLodge`, `AtParking`)
  matches `FromID`.
- **Apply:** clears the old anchor; sets `AtLiftBase`, `AtLodge`, `AtParking`, or `AtTrailEnd`
  depending on `ToKind`.
- **Cost:** `Distance / skiSpeedMps` (10 m/s).

`trailActions()` (called from `ApplicableActions`) looks up all edges from the current anchor
via `w.TrailGraph.EdgesFrom` and emits one `SkiTrail` per edge.

**Off-trail penalty** added to free-roam `SkiToLift / SkiToLodge / SkiToParking` when trail
alternatives exist:

| Skill | Penalty |
|---|---|
| Beginner | +120 s |
| Intermediate | +45 s |
| Advanced | +10 s |
| Pro | 0 s |

`AtTrailEnd uint64` on `WorldSnapshot` tracks trail-junction arrivals; included in `stateKey()`;
cleared on `JoinQueue`, `WalkToLift`, `RideLift`.

`ActSkiTrail` is the action kind; `PlanAction` carries `TrailID`, `FromID`, `ToID`
(`internal/ai/types.go`).

**`internal/sim/simulation.go` — `onPlanStepStart`** resolves `ActSkiTrail` to a world target
and goal. On step completion, sets `a.AtTrailEnd = head.TrailID` for trail-to-trail chaining.

---

## Trail painting UI

**`internal/scene/scenario.go`**

Tool mode `toolTrailPaint`. Brush radius: `trailPaintBrushRadius = 2` cells (~10 m).

- **Left-drag:** paint cells onto the active trail (`applyTrailPaint`).
- **Right-drag:** erase cells (`applyTrailErase`).
- **Drag end:** `finishTrailPaintStroke()` calls `RebuildTrailGraph()`.
- **Click trail label:** select that trail for editing.
- **Escape / switch tool:** commits trail, exits paint mode.

New trail created via `w.PlaceTrail(name, diff)` with auto-name ("Run 1", "Run 2", …).
Deletion via `w.DeleteTrail(id)` — replans any guests whose current step referenced the trail.

---

## Rendering

**Overlay (toggleable):** trail cells tinted on the terrain by difficulty color (~0.4 alpha):
- Green `#55AA55` / Blue `#4488CC` / Black `#333333`.
- Overlapping cells show the higher-difficulty color.
- `BuildTrailOverlay(w)` generates the per-cell color array; rebuilt on trail change.

**Name labels (default on, toggleable):** trail name rendered at `trail.Centroid()` in
world-space using the existing `Font` renderer, difficulty-colored. Toggled via the Overlays panel.

---

## Save / load

**`internal/save/format.go`**

```go
type TrailData struct {
    ID         uint64   `json:"id,omitempty"`
    Name       string   `json:"name,omitempty"`
    Difficulty uint8    `json:"diff,omitempty"`
    Cells      [][2]int `json:"cells,omitempty"`
}
```

`TrailGraph` is **not** saved — rebuilt by `dataToWorld` after all trails are restored.
Save format is msgpack + gzip (`.save` in `~/.mountain-mogul/saves/`).

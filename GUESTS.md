# Guest AI — pipeline overview

The guest AI is split into two layers:

- **L0 — strategic layer**: a per-agent Goal-Oriented Action Planning
  (GOAP) loop in `internal/ai/goap/`. It picks goals and chains actions;
  the head action's destination is the goal target for L1.
- **L1–L3 — continuous controller**: perception → steering decision →
  physics integration in `internal/sim/skiing.go`, running every tick.
  Reactive: given a goal target, ski toward it.

L0 is intentionally thin compared with L1; the per-tick controller never
re-reads strategic state mid-tick. Replanning happens between ticks on
explicit triggers — never as a fixed-interval poll, never per-frame.

Persistent per-guest state on `world.Guest`: `Traits`, `Plan` (the L0
stored plan + L1 goal target), `Balance`, `TurnSide`, `TurnDwell`,
`LastTactical`, `Patience`, `PositiveThoughts`, `NegativeThoughts`,
`RidenLifts`, `RestTimer`, `Removed`, `Sense`. Per-tick types
(`Perception`, `Decision`) are sim-internal and never stored.

---

## L0 · Strategic layer — what's in the tree

### GOAP in one paragraph

A planning architecture from F.E.A.R. (Orkin, 2005). Each agent has a
typed **world snapshot**, a small library of **actions** (each with a
precondition, an effect, and a cost), and a small set of **goals** (each
with a "satisfied when" predicate and a weight). At replan time `SelectGoal`
picks the highest-weighted unsatisfied goal, and an A\* search over the
action graph finds the cheapest chain that satisfies it. The output is a
list of actions; the agent executes the head, advances when its effect is
realised, and re-plans when its precondition breaks. New behavior is one
action plus one goal — no state-machine rewrite per combination.

### World snapshot

Per-agent typed snapshot extracted at replan time via `goap.Extract`.
ID-valued where the field is categorical; numeric where natural.

```go
type WorldSnapshot struct {
    Pos        mgl32.Vec3
    Patience   float32          // 0..1 — drains queuing, restored by skiing/riding/lodge
    AtLiftBase uint64           // 0 or lift ID
    AtLiftTop  uint64           // 0 or lift ID
    Queued     uint64           // 0 or lift ID
    OnLift     uint64           // 0 or lift ID
    AtLodge    uint64           // 0 or lodge building ID
    AtParking  uint64           // 0 or parking building ID
    Removed    bool             // terminal — agent has Departed
    RidenLifts map[uint64]int   // per-lift ride count (novelty driver)
}
```

Anchor IDs are populated by proximity to known anchors (8 m radius), with
implicit state markers (`OnLiftID`, `Queued`) overriding proximity. An
agent in transit between anchors lands with all positional IDs zero —
that's the "no anchor" state the planner sees mid-descent. Action
preconditions are usually one comparison; effects are usually one
assignment.

### Actions (8)

| Action | Precondition | Effect (planner-side) | Base cost |
|---|---|---|---|
| `WalkToLift(L)` | no anchors, not on/queued for a lift | `AtLiftBase = L` | `dist(Pos, L.Base) / WalkSpeed` |
| `JoinQueue(L)` | `AtLiftBase == L` | `Queued = L`; `AtLiftBase = 0` | `len(L.Queue) × queueSlotSec` |
| `RideLift(L)` | `Queued == L` or `OnLift == L` | `AtLiftTop = L`; `OnLift = 0`; increment `RidenLifts[L]` | `L.LoopLength / (2·L.Speed)` + repeat penalty |
| `SkiToLift(L)` | `AtLiftTop != 0`; ≥20 m descent to `L.Base` | `AtLiftBase = L`; `AtLiftTop = 0` | `dist / skiSpeedMps` |
| `SkiToLodge(B)` | `AtLiftTop != 0`; ≥20 m descent to `B` | `AtLodge = B`; `AtLiftTop = 0` | `dist / skiSpeedMps` |
| `SkiToParking(B)` | `AtLiftTop != 0`; ≥20 m descent to `B` | `AtParking = B`; `AtLiftTop = 0` | `dist / skiSpeedMps` |
| `RestAtLodge(B)` | `AtLodge == B` | `Patience = 1` | `restDurationSec` (≈60 s) |
| `Depart(B)` | `AtParking == B` | `Removed = true` | `0` (terminal) |

Boarding the chair is folded into `RideLift` — no separate `BoardChair`
step, because there's no game-state effect between queue front and chair
seat the planner would condition on.

**Trail-free reachability.** `SkiTo*` is gated on a minimum elevation
drop (`minDescentMeters = 20 m`) between the source lift top and the
destination, as a stand-in for player-defined trails. Cost is straight-
line distance divided by `skiSpeedMps` — the L1 controller decides the
actual path through terrain.

**Novelty mechanic.** `RideLift.Cost` adds a linear repeat penalty
(`repeatPenaltyPerRide × prior_count`, capped at `repeatPenaltyCap`) so
unridden lifts plan as cheaper. On unload `RidenLifts[L]++` is
incremented, and on the first-ever ride of a lift `ThoughtLovingALift`
(positive) is emitted — so the planner's preference for unridden lifts
matches the actual rating outcome.

### Goals (4)

| Goal | Satisfied when | Weight |
|---|---|---|
| `KeepSkiing` | `AtLiftTop != 0` | `Patience` (or `Patience × 0.5` if `Patience < 0.2`) |
| `Rest` | `Patience ≥ 0.85` | `(1 − Patience)²` |
| `Explore` | every lift in `RidenLifts` with count > 0 | `unridden_frac × Patience` |
| `GoHome` | `Removed` | `1.0` if `Patience < 0.05`, else `0` |

`SelectGoal` returns the highest-weighted unsatisfied goal. Low-patience
routing falls out of standard weight evaluation — no special-case branch.

**Note on `KeepSkiing.IsSatisfied = (AtLiftTop != 0)`.** This is a
planner-terminal predicate — "the plan must end at a lift top." It's
transient at runtime: as soon as the skier leaves the top, KeepSkiing
goes unsatisfied. That's intentional given current triggers (the plan in
flight runs to completion without re-electing), but it's the reason the
periodic safety re-check was removed — a wall-clock re-check on a
transient snapshot loops skiers back to the lift indefinitely.

### Plan storage on `world.Guest`

The plan lives on `guest.Plan` as plain data in the leaf `internal/ai`
package (no goap import from `world`, avoiding a cycle):

```go
type Plan struct {
    Goal     GoalKind     // L1 hint: GoalLift / GoalDepart / GoalNone
    GoalID   uint64       // entity the L1 controller is steering toward
    Target   mgl32.Vec3   // L1 target world pos
    GoalName string       // L0 goal name for HUD ("Explore" / "Rest" / ...)
    Steps    []PlanAction // the L0 plan
    Step     int          // index of the current head action
    Prefs    Prefs        // reserved
}

type PlanAction struct {
    Kind   PlanActionKind  // ActWalkToLift / ActJoinQueue / ...
    LiftID uint64          // 0 unless lift-typed
    BldgID uint64          // 0 unless building-typed
    Cost   float32         // planner cost-at-emission, for HUD
}
```

`goap.ToPlanActions` translates the planner's `[]Action` output into
`[]PlanAction` (one switch on concrete type per step). `Planner.StoredPlanFor`
is the runtime entry point — extract snapshot, select goal, plan, translate.

### Replan triggers

1. **Plan empty** — agent just spawned, or the previous plan exhausted.
2. **Head action complete** — fresh snapshot matches the action's post-
   state (`WalkToLift` complete when `snap.AtLiftBase == L`, etc).
3. **Head action precondition broken** — entity referenced by the head
   step no longer exists. Runtime check is laxer than the planner's
   GOAP precondition (just "entity exists"); the stricter GOAP form
   would constantly fail during transit (`SkiToLodge` wants `AtLiftTop != 0`
   which is gone as soon as the skier moves off the top), forcing
   constant replanning.

There is **no periodic safety re-check.** Future "world changed under
the agent" coverage — lift closure, queue spike crossing a threshold,
affect threshold — should land as explicit event hooks that mark
affected agents `Stale` so their next tick replans. A wall-clock poll
collides with transient snapshot predicates and is worse than nothing.

### Pipeline diagram (L0)

```mermaid
flowchart TB
  Tick([per-agent tick]) --> Trig
  Trig{plan empty?<br/>head done?<br/>head precondition broken?}
  Trig -->|no| Exec
  Trig -->|yes — empty/broken| Replan
  Trig -->|yes — head done| Advance

  Replan["<b>StoredPlanFor</b><br/>Extract → SelectGoal → A* → ToPlanActions"]
  Advance["<b>advancePlan</b><br/>Step++ · re-enter at new head"]
  Replan --> Start
  Advance --> Start

  Start["<b>onPlanStepStart</b><br/>materialise step effect on live world<br/>(TargetID, pathfinder route, queue, RestTimer, Removed)"]
  Start --> Exec

  Exec["<b>per-tick dispatch</b><br/>tickRiding / tickQueued / tickResting /<br/>tickPath / tickLocomote · then reapDeparted"]

  classDef stage fill:#0f172a,stroke:#475569,color:#e2e8f0
  classDef gate fill:#1f2937,stroke:#9ca3af,color:#f3f4f6
  class Replan,Advance,Start,Exec stage
  class Trig gate
```

`tickPlanning(agent)` runs first per agent each frame and dispatches one
of the three branches. `onPlanStepStart` is the single point that maps
each `PlanAction.Kind` to live sim state — laying pathfinder routes for
walks, appending to lift queues, starting rest timers, marking departed
agents for the post-loop reap. `reapDeparted` splices `Removed` agents
out after the loop so range iteration doesn't shift mid-pass.

### Spawn and load

`tickBuildings` spawns an agent and calls `s.replan(agent)`. The first
step (typically `WalkToLift`) drives `onPlanStepStart` to lay a path
from the parking lot's door to the lift's queue back. Spawn unwinds if
either the planner returns no plan or the pathfinder can't reach the
target.

`NewSimulationWithSeed` walks any pre-existing agents (testbeds, save-
restored) and calls `onPlanStepStart` for each non-empty plan so the
runtime state matches the stored plan before the first tick. Save files
don't serialise `Plan.Steps` — on load the plan is empty and tickPlanning's
plan-empty branch regenerates it from the agent's snapshot.

### HUD

- **followLabel** (centred banner): identity, speed/energy/fun, head
  action's display name.
- **plannerDebugPanel** (F4-toggled): goal-weight ranking computed from
  a fresh snapshot, snapshot anchors, `RidenLifts`, the full stored plan
  with a `→` marker on the current step.

Both read `agent.Plan` directly — never call the planner from the draw
path. The goal-weights table re-extracts a snapshot each frame for the
followed agent only, which is microseconds.

### Lift naming

`PlaceLift` assigns `Lift1`, `Lift2`, ... via `world.nextLiftDefaultName`
at creation time. Players can rename through the lift popup. Plan
readouts use `goap.PlanActionLabel` which resolves IDs to the lift's
name (or `#ID` fallback for unnamed) and to `Lodge#X` / `Lot#X` for
buildings.

---

## L1–L3 · Continuous controller

Continuous steering controller. The pipeline runs once per agent per tick
from `tickSkier` in `internal/sim/skiing.go`. There is **no technique
enum** and **no waypoint planner** — S-turns, brake wedging, and tree
avoidance all emerge from a single steering function reading a small
typed perception bundle.

```mermaid
flowchart TB
  Start([tickSkier · per agent · per tick])

  Start --> Arrival{dist &lt; ArrivalThreshold?}
  Arrival -->|yes| Done([snap to target · return arrived])
  Arrival -->|no| L1

  subgraph L1["L1 · Perception — every tick"]
    direction LR
    Terrain["<b>Terrain</b><br/>NormalAt (snow surface) · SlopeAngle<br/>FallDir · FallScale (smoothstepped [0,1])"]
    Axis["<b>Axis target</b><br/>vector to a.Plan.Target<br/>→ AxisDir · AxisDist · InArrival (&lt;6 m)"]
    Under["<b>Underfoot</b><br/>TreeDensityAt(pos) → AtCellDensity<br/>InTrees flag (display only)"]
  end

  L1 --> L2

  subgraph L2["L2 · Controller (decide) — Perception → Decision"]
    direction TB
    Axis2["<b>composeAxis</b><br/>blend(AxisDir, FallDir·FallScale)<br/>→ axisHeading"]
    Tact["<b>sampleTactical</b><br/>7 candidate offsets · ±60° · 8 segments<br/>horizon = speed·3.5 s clamped [22, 70] m<br/>corridor ±10 m perpendicular — worst-of-three density<br/>score = progress·cos(off) − 4·Σdensity − 8·boundaryHits<br/>+ 0.5·Σgrooming  <i>(centre-only)</i><br/>+ 0.4·sign(prev)·sign(off)  <i>(gated on maxDensity&gt;0.5)</i><br/>+ tiny RNG jitter to break symmetric ties"]
    Speed["<b>targetSpeed</b><br/>ComfortSpeed·(0.7 + 0.6·Aggression)<br/>× 0.5 if InArrival<br/>× (1 − 0.4·clamp(worstProbe/0.4))<br/>floor at skiWalkSpeed (2 m/s)"]
    Brake["<b>brakeAngle</b><br/>overspeed = (Speed − target)/target<br/>brakeAngle = clamp(overspeed·1.5, 0, 40°)"]
    Side["<b>TurnSide commit</b> — persistent, dwell-gated<br/>dropped to 0 when obstacleSeen OR brakeAngle &lt; 8°<br/>flips to opposite when |deviation| &gt; 0.85·brakeAngle<br/>AND TurnDwell ≥ 1.2 s<br/>initial side from heading deviation, else coin-flip"]
    Out["<b>desiredHeading</b> = axisHeading + tactical + side·brakeAngle<br/><b>scrub</b> = 4·(overspeed − 0.6) clamped to 6 m/s²<br/>active only above 60% overspeed"]
    Axis2 --> Tact --> Speed --> Brake --> Side --> Out
  end

  L2 --> L3

  subgraph L3["L3 · Apply — physics integration"]
    direction TB
    Head["<b>heading</b><br/>rotateToward(desired) capped at headingRateMax (40°/s)<br/>— matches realistic edge-transfer rate"]
    Fric["<b>effectiveFriction</b> — snow-modulated (muBase, muEdge)<br/>Grooming · Packed · Powder gate · MogulSize · Ice<br/>each shifts the corduroy baseline (see SNOW.md)"]
    Accel["<b>acceleration</b><br/>a = g·sinθ·cos(off) <br/>− μ_base·g·cosθ<br/>− μ_edge·g·cosθ·|sin(off)|<br/>− k_drag·v²<br/>− dec.Scrub"]
    Pos["<b>position</b><br/>pos.xz += (sin h, cos h)·speed·dt<br/>pos.y = surface elevation<br/>floor speed ≥ skiWalkSpeed (2 m/s)"]
    Head --> Fric --> Accel --> Pos
  end

  L3 --> BalChk{Balance += stressDelta·dt<br/>≤ 0?}
  BalChk -->|yes| Fallen["<b>Fallen</b> · 4 s<br/>resume at Balance 0.7 · TurnSide 0"]
  BalChk -->|no| Snap["<b>recordFrame → Sense</b><br/>follow HUD · perception-cone shader · CSV recorder"]

  classDef layer fill:#0f172a,stroke:#475569,color:#e2e8f0
  classDef phys fill:#7c2d12,stroke:#fb923c,color:#fed7aa
  classDef gate fill:#1f2937,stroke:#9ca3af,color:#f3f4f6
  class L1,L2 layer
  class L3 phys
  class Arrival,BalChk gate
```

### Notes on the architecture

- **Plan A — no technique enum.** Straight, carved, and brake-heavy outputs
  all come from one steering function. The brake angle (`TurnSide ×
  brakeAngle`) is what produces emergent S-turns: while overspeed,
  brakeAngle > 0 → desired heading is off the fall line → edge friction
  scrubs speed → speed drops → brakeAngle shrinks → if heading has reached
  the arc edge on the committed side, flip TurnSide and carve back.
- **No path planner.** `a.Plan.Target` tracks the L1 goal target, set
  once by `onPlanStepStart` per L0 step. There are no waypoints, no
  routes inside L1. The controller seeks the goal directly and lets
  `sampleTactical` deal with obstacles in front of it.
- **Single forward sampler.** `sampleTactical` scores 7 candidate arcs at
  ±60° around `axisHeading`. Each arc is 8 segments deep; every segment
  reads tree density at the centre **and** at ±10 m perpendicular, taking
  the worst — so a path that grazes a tree edge scores as poorly as one
  through the trunk. Boundaries get an 8× penalty, tree density a 4×
  penalty, on-axis progress a +0.3 bonus.
- **Side-commit on obstacles only.** A small bias (+0.4 × sign(prev) ×
  sign(off)) keeps the skier on the same side they chose last tick — but
  only when the fan actually sees an obstacle (`maxDensity > 0.5`). Without
  that gate, `prevTactical` would self-perpetuate and slowly drift the
  skier off-axis even on a clear slope.
- **S-turn suppression while avoiding.** When `obstacleSeen`, TurnSide is
  forced to 0 — the tactical offset already takes the heading off the fall
  line, so cross-fall friction still scrubs speed, and the S-turn
  oscillation would otherwise fight the lateral commitment by swinging
  heading back through axis every cycle. Real skiers don't S-turn through
  trees.
- **Turn dwell minimum.** A committed turn side can't flip again until 1.2 s
  has passed (`turnDwellMin`). Combined with the 40°/s heading rate cap
  this puts each carve at ~1.2 s minimum and a full S-cycle at ~2.4 s —
  cruising rhythm, not slalom.
- **Snow-modulated friction.** `effectiveFriction` reads `SnowAt(pos)` and
  shifts the (muBase, muEdge) pair per Grooming / Packed / Powder /
  MogulSize / Ice. See `SNOW.md` for the multiplier table.
- **Grooming preference in steering.** `sampleTactical` integrates per-
  segment `Grooming` along each candidate arc (centre-only — the edge of
  a groomed strip is still groomed) and adds `0.5 · Σgrooming` to the
  score. On clear slopes this pulls the line onto corduroy; when trees
  are present the 4× density penalty dominates and the grooming term
  just biases tie-breaks. Uniform across skiers — `GroomingPreference`
  trait is deferred.
- **Balance + fall** runs every tick orthogonally to L1–L3. Drains from
  speed/slope overshoot, hard scrub under load, and underfoot tree density
  above 0.3. Recovers at +0.15/s baseline, clamped to [-1, 0.4]/s.
- **Patience** is the session frustration budget. Drains only while
  queuing (`−1/600` per sim-second); restored by active skiing
  (`+1/1000` per sim-second), riding (`+1/800` per sim-second), and
  instantly by `RestAtLodge`. The L0 `Rest` goal fires at low patience
  (`Rest.Weight = (1 − Patience)²`), producing a `SkiToLodge +
  RestAtLodge` plan. `GoHome` fires when Patience < 0.05. The skier
  physics pipeline never reads Patience itself.

---

## Per-session stats, rating, and thoughts

### Patience

`Patience` (0..1) is the guest's tolerance budget for the session. It starts
at 1.0 on arrival and is the sole stat the L0 planner reads to weight goals.
When it reaches 0, `GoHome` fires and the guest leaves.

**Write sites:**

| Source | Rate | Activity |
|---|---|---|
| Queuing in a lift queue | `−1/600` per sim-second | drains: ~10 min of pure queuing exhausts budget |
| Active skiing (`tickSkier`) | `+1/1000` per sim-second | restores slowly from fun descents |
| Riding a lift chair (`tickRiding`) | `+1/800` per sim-second | restores: chair ride offsets earlier wait |
| Lodge rest (`tickResting`) | instant `= 1` | full restore on `RestAtLodge` completion |

Patience is clamped to `[0, 1]` on every write.

### Rating

`Guest.Rating()` returns a 0..1 score derived from the session thought counts.
Read mid-session by the demand system's daily poll and stamped on `LastScore`
at departure.

```
score = PositiveThoughts / (PositiveThoughts + NegativeThoughts)
```

Returns 0.5 when no thoughts have fired yet. A session full of positive
terrain moments and novel lift rides scores near 1; a session dominated by
falls and long queues scores near 0.

### Thoughts

The thoughts ring is the player-visible "what's this guest thinking" surface.
It holds up to 6 entries (`thoughtsCap`) in a fixed-size ring buffer; the
oldest slot is recycled when full.

**`AddThought(kind, simTime)`** suppresses duplicates: if the same `ThoughtKind`
is already in the ring and its timestamp is within `ThoughtTTL = 12 s`, the
new push is dropped. Each push that isn't suppressed increments
`PositiveThoughts` or `NegativeThoughts` on the guest.

**`CurrentThought(simTime)`** returns the most-recent unexpired thought by
walking the ring backwards from the write head. Returns `ThoughtNone` when
the ring is empty or all entries are older than 12 s.

**Emitted thoughts:**

| Kind | Polarity | Display | When emitted |
|---|---|---|---|
| `ThoughtLovingGlades` | ✅ positive | "loving these glades" | `LikesGlades` + in trees (per tick, TTL-gated) |
| `ThoughtScaredInTrees` | ❌ negative | "too many trees!" | non-glade guest in trees (per tick, TTL-gated) |
| `ThoughtLovingCorduroy` | ✅ positive | "this corduroy is perfect" | `PrefersGroomed` + on groomed (per tick, TTL-gated) |
| `ThoughtTiredOffPiste` | ❌ negative | "this snow is exhausting" | `PrefersGroomed` + off-piste (per tick, TTL-gated) |
| `ThoughtFell` | ❌ negative | "ouch, that hurt" | balance reaches 0 (per fall, TTL-gated) |
| `ThoughtLovingALift` | ✅ positive | "what a great lift!" | first ride of a previously-unridden lift |
| `ThoughtLongLine` | ❌ negative | "this line is way too long" | joining a queue of ≥ 15 people |

---

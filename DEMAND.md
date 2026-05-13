# Demand system — global skier pool + resort rating

The demand system is the source of skiers in the simulation. It owns a
fixed-size **catchment pool** partitioned by skill level, polls each
group every ~30 sim-seconds to decide whether to send a carload, and
maintains a 0..1 **resort rating** that summarises recent guests'
experiences. Departing skiers score their session (Fun, Energy, fall
ratio) and their score EMAs into the rating; the rating is one of the
three multiplicands in the visit probability, so a happy resort attracts
more guests automatically.

Lives at `internal/sim/demand.go`, owned by `Simulation`. The lot itself
is passive — `MaxCars` caps the visible population, `CurrentCars` is
what the renderer floors to N car meshes; both written by the demand
system, neither by the lot.

---

## Pipeline (high-level)

```
                        every demandPollInterval sim-seconds
                                       │
                                       ▼
                               ┌──────────────┐
                               │  maybePoll   │
                               │ (capacity,   │
                               │  occupancy)  │
                               └──────┬───────┘
                                      │ for each skill group:
                                      │   p = rating × terrainMatch × (1 − occ)
                                      │   if Bernoulli(p) → spawn 1 carload
                                      ▼
                            ┌──────────────────┐
                            │   spawnSkier ×4  │  (one carload)
                            └────────┬─────────┘
                                     │ Pool -= 4, lot.CurrentCars += 1
                                     ▼
                            ┌──────────────────┐
                            │ active sim agent │  L0 plan drives session
                            │ (skis, queues,   │  events appended:
                            │  rides, rests)   │  EventFall / EventRun
                            └────────┬─────────┘
                                     │ planner runs ActDepart
                                     ▼
                            ┌──────────────────┐
                            │ recordDeparture  │  score = α·Fun + β·Energy
                            │   → rating EMA   │        + γ·cleanness(falls, runs)
                            │   → pool return  │  Pool += 1, lot.CurrentCars -= 1/4
                            └──────────────────┘
```

---

## State

```go
type DemandSystem struct {
    Groups       [3]SkierGroup // indexed by ai.SkillLevel
    ResortRating float32        // 0..1 EMA, init 0.5
    LastPoll     float64        // sim-time of last poll
}

type SkierGroup struct {
    Pool int  // skiers in this group still available to draw from
}
```

Initial pool sizes at simulation start are `{6000, 3000, 1000}` —
mirrors the 60 / 30 / 10 % skill distribution from the old
`rollSkillLevel` at season-scale (10 k skiers in the catchment).

`world.Agent` carries the per-session event log:

```go
type AgentEvent struct {
    Kind AgentEventKind  // EventFall | EventRun
    Time float64         // sim-time of emission
}

// world.Agent
Events []ai.AgentEvent
```

Emitted at two points in the sim:
- **`EventFall`** in `tickSkier`'s balance-zero branch
  (`internal/sim/skiing.go`).
- **`EventRun`** in `tickPlanning` whenever an `ActSkiToLift` /
  `ActSkiToLodge` / `ActSkiToParking` step's completion fires — one
  event per descent (`internal/sim/simulation.go`).

The log is read once at depart and then discarded with the agent.

---

## The poll

`DemandSystem.maybePoll(s *Simulation)` runs from `Simulation.Tick`
once per frame; it short-circuits unless
`s.SimTime - LastPoll >= demandPollInterval` (currently **30 sim-seconds**).

```go
capacity = Σ lifts: chairs × seats × (1 / loopTime) × avgSessionSec
occupancy = len(World.Agents) / capacity

for each group (skill):
    if Pool < SkiersPerCar:          continue   // catchment exhausted
    if terrainMatch(group) == 0:     continue   // no lifts they'd ride
    p = clamp01(rating × match × (1 − clamp01(occupancy)))
    if rng.Float32() >= p:            continue
    lot = uniform-random parking
    for 1..SkiersPerCar:              spawnSkier(lot, group)
    Group.Pool -= spawned
    lot.CurrentCars += 1
```

**Capacity** (`resortCapacity`) is the "comfortable skiers-at-once"
estimate. It's a function of the player's installed lifts only — more
chairs, faster lifts, longer lifts all raise capacity. `avgSessionSec`
(currently 800 s — same as the L1 Energy budget) is the typical time a
skier-seat is occupied across one cycle. The formula reduces to
`Σ chairs × seats × sessionLengthsPerLoop` — adding a second lift
doubles capacity.

**Occupancy** is the inverse of headroom: `1 − occ` falls to zero as
the resort fills, throttling demand without a hard cap. The crowd
self-limits before the renderer chokes on agent count.

**`terrainMatch`** is binary today: 1 if any lift's `Services` bitfield
includes the group's matching difficulty (Beginner→Green,
Intermediate→Blue, Advanced→Black), else 0. A green-only resort
attracts beginners but no intermediates / advanced. Smoother forms
(fraction of matching lifts, weighted by trail length) become the
obvious next iteration once trails are first-class.

**`visitProbability`** is multiplicative on purpose — any factor near
zero kills demand (no rating → nobody comes; no matching terrain →
that group skips; full resort → throttles). Sigmoidal forms would be
flexible but the multiplicative shape is easier to reason about and
the three factors are each independently meaningful.

---

## Spawning

`Simulation.spawnSkier(lot, skill)` (private helper in `simulation.go`)
replaces the old `tickBuildings` spawn block:

1. Create a `world.Agent` at the lot's door cell, with traits from
   `ai.TraitsFor(skill)`, `Balance = 1.0`, `Energy = 1.0`.
2. Append to `world.Agents`.
3. Call `s.replan(agent)` — the L0 planner picks the first lift and
   `onPlanStepStart(ActWalkToLift)` lays a pathfinder route from the
   lot to the chosen lift's queue back.
4. If the planner returns no plan OR `WalkToLift` couldn't lay a path
   (pathfinder failed), unwind: remove the agent and return `false`.

The caller (`maybePoll`) counts successes and only deducts from the
group pool / bumps `CurrentCars` by the number of skiers that actually
made it into the world.

---

## Departure → rating

The GOAP `ActDepart` action runs through `onPlanStepStart` like any
other plan step. The depart branch:

```go
case ai.ActDepart:
    s.Demand.recordDeparture(a)         // score + EMA + pool return
    lot.CurrentCars -= 1 / SkiersPerCar // 4 departures = -1 car
    a.Removed = true                    // reaped after the tick loop
```

`recordDeparture(a)`:

1. **Score** the session:

```go
falls = count(a.Events, EventFall)
runs  = count(a.Events, EventRun)

cleanness = 1 - clamp(falls / max(runs, 1), 0, 1)   // 1 = no falls
score = α·Fun + β·Energy + γ·cleanness               // α+β+γ = 1
```

   Weights are `{0.4, 0.3, 0.3}`. A perfect session (`Fun = 1`,
   `Energy = 1`, no falls) scores 1.0; a disastrous one (`Fun = 0`,
   `Energy = 0`, all falls) scores 0.0. The score is clamped to
   `[0, 1]` after the weighted sum.

2. **EMA into rating** with a slow half-life:

```go
ResortRating ← (1 − λ)·ResortRating + λ·score    // λ ≈ 1/70 ≈ 0.014
```

   ~50-departure half-life — slow enough that one grumpy guest can't
   tank the score, fast enough that a player's edit (better lift,
   groomed terrain) shows up in arrivals within a session.

3. **Return** the skier to their group's pool: `Groups[skill].Pool++`.
   The pool conserves — the only way to lose a skier permanently is
   for the spawn to fail (planner/pathfinder) and the unwind to skip
   the deduction.

---

## HUD surface

The top bar's happiness bar reads `s.sim.Demand.ResortRating` directly
(`internal/scene/scenario.go`). The bootstrap value (0.5) shows a
neutral half-bar on a fresh scenario; the bar drifts up or down as
guests depart with good or bad sessions.

Per-group pool sizes aren't surfaced — they need a dedicated info
window that doesn't exist yet.

---

## Tunables (in `demand.go`)

| Constant | Default | Effect |
|---|---|---|
| `SkiersPerCar` | 4 | Carload size; one demand-poll success spawns this many skiers and adds 1 car visually |
| `demandPollInterval` | 30 s (sim) | How often the poll fires; the future time/weather system will modulate this |
| `ratingEMAλ` | 1/70 ≈ 0.014 | EMA blend factor; ~50-departure half-life for ResortRating |
| `initialResortRating` | 0.5 | Bootstrap value on fresh sim |
| `avgSessionSec` | 800 | Capacity estimate's "skiers-at-once" denominator; matches L1 `energyBudgetSec` |
| `scoreWeightFun` | 0.4 | α — weight of `Fun` in the departure score |
| `scoreWeightEnergy` | 0.3 | β — weight of remaining `Energy` |
| `scoreWeightClean` | 0.3 | γ — weight of `1 − falls/runs` |

Initial pool sizes live in `NewDemandSystem` rather than as constants
so they can vary by scenario later.

---

## Tests

`internal/sim/demand_test.go` covers the pure-function surface:

- **`TestVisitProbability`** — locks the multiplicative shape (any zero
  factor kills demand; clamping behaves).
- **`TestScoreDepartureExtremes`** — perfect session → 1.0, disaster
  → 0.0.
- **`TestScoreDepartureNoRuns`** — guards the divide-by-zero in
  cleanness when no descents happened.
- **`TestTerrainMatchBinary`** — the skill → difficulty bit lookup.

---

## What's not in here (yet)

- **Per-group preferences / loyalty.** Today the only group axis is
  skill. Eventually skiers could remember their last visit's rating,
  prefer specific resorts, or arrive in family-shaped clusters.
- **Time-of-day / weather modulation.** The poll fires at a flat rate.
  A clock + weather system would shape the curve (morning peak, snow
  storm boost / penalty, season).
- **Per-lot draw weighting.** New skiers pick a uniform-random parking
  lot. Lot occupancy, distance to lifts, or pricing could weight the
  draw.
- **Richer `terrainMatch`.** Binary today. The fraction-of-matching
  lifts form (or trail-aware) lands once trails are first-class
  entities.
- **Per-group pool dynamics.** Pool sizes are static at season start.
  A reputation system could let the catchment grow / shrink with the
  resort's long-run rating.
- **Rating display.** Top-bar shows the float as a happiness bar. A
  dedicated info window (planned) would surface per-group pool sizes,
  recent depart scores, and the rating's trend.

The shape is set up so each of these is local — a new field on the
group, a new factor in `visitProbability`, or a new term in
`scoreDeparture` — without restructuring the pipeline.

# Demand system — global skier pool + resort rating

The demand system is the source of skiers in the simulation. It walks
the fixed **catchment pool** (`world.World.Guests`) every ~30 sim-seconds
and rolls a Bernoulli per `AtHome` guest to decide whether they arrive.
It maintains a 0..1 **resort rating** (`ResortRating`) that is the EMA of
departing guests' `Satisfaction` scores; rating is one of the multiplicands
in the visit probability, so a happy resort attracts more guests automatically.

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
                                      │ for each AtHome guest:
                                      │   skip if can't afford day ticket
                                      │   p = rating × terrainMatch × (1 − occ) × priceFactor
                                      │   if Bernoulli(p) → spawnGuest
                                      ▼
                            ┌──────────────────┐
                            │   spawnGuest     │  Satisfaction = 0.6, Patience = 1
                            │                  │  day ticket → Cash, revenue
                            └────────┬─────────┘
                                     │ lot.CurrentCars += 1/GuestsPerCar
                                     ▼
                            ┌──────────────────┐
                            │ active sim agent │  L0 plan drives session
                            │ (skis, queues,   │  Satisfaction drifts + spikes
                            │  rides, rests)   │  per tick and on events
                            └────────┬─────────┘
                                     │ planner runs ActDepart
                                     ▼
                            ┌──────────────────┐
                            │ recordDeparture  │  LastScore = Satisfaction
                            │   → rating EMA   │  ResortRating EMA update
                            │                  │  lot.CurrentCars -= 1/GuestsPerCar
                            └──────────────────┘
```

---

## State

```go
type DemandSystem struct {
    ResortRating float32  // 0..1 EMA of departing guests' Satisfaction; init 0.5
    LastPoll     float64  // sim-time of last per-guest poll pass
}
```

The catchment itself is `world.World.Guests` — a flat slice of `*world.Guest`
pointers. Each guest has `VisitsPerSeason` (expected annual visits) and
`State` (AtHome / OnMountain). The demand system never owns guest identity;
it just flips State and populates sim scratch fields at spawn.

---

## The poll

`DemandSystem.maybePoll(s *Simulation)` runs from `Simulation.Tick`
once per frame; it short-circuits unless
`s.SimTime - LastPoll >= demandPollInterval` (currently **30 sim-seconds**).

```
capacity = Σ lifts: chairs × seats × (1 / loopTime) × avgSessionSec
occupancy = len(World.OnMountain) / capacity

for each g in World.Guests where g.State == AtHome:
    priceFactor = dayTicketPriceFactor(g, rating)
    if priceFactor == 0: continue   // day ticket above their budget
    match = terrainMatch(g.Traits.Skill)
    if match == 0: continue   // no lifts they'd ride
    dailyRate = g.VisitsPerSeason / seasonDaysApprox
    p = dailyRate × pollFraction × rating × match × (1 − occupancy) × priceFactor
    if rng.Float32() >= p: continue
    lot = uniform-random parking lot
    spawnGuest(lot, g)
    lot.CurrentCars += 1 / GuestsPerCar
```

**Capacity** (`resortCapacity`) is the "comfortable skiers-at-once"
estimate. More chairs, faster lifts, and longer lifts all raise it.

**Occupancy** throttles demand without a hard cap — the crowd self-limits.

**`terrainMatch`** is binary: 1 if any lift's `Services` bitfield includes
the guest's skill difficulty (Beginner→Green, Intermediate→Blue,
Advanced→Black), else 0.

**`visitProbability`** is multiplicative — any factor near zero kills demand.

**`dayTicketCharge(w, g, simTime)`** prices the visit. It returns
`(price, ok)`: a guest with a valid season pass pays 0 and is always ok;
everyone else pays `World.DayTicketPrice` and is ok only if
`Traits.DailyBudget` covers it.

**`dayTicketPriceFactor(w, g, simTime, rating)`** is the price elasticity
term, in [0, 1]:

```
ref = DayTicketReferencePrice × (1 + DayTicketRatingPremium × (rating − 0.5))
priceFactor = 0                                          if price > budget
            = 1                                          if price ≤ ref (or pass holder)
            = ((budget − price) / (budget − ref))^DayTicketElasticity   otherwise
```

Rating shifts the reference rather than scaling the factor: with the
defaults ($60, premium 1.0) the no-penalty price runs from $30 at rating 0
through $60 at 0.5 to $90 at 1.0. A well-rated resort can charge more
before guests balk. Rating also still multiplies `p` directly, so it
counts twice: once for "is this place any good" and once for "is it worth
the price". Elasticity 0.5 bows the curve, so a guest keeps most of their
interest until the price gets close to their budget.

Worked numbers for the seeded catchment (`DailyBudget = 40 + 160 × skill`,
60/30/10 beginner/intermediate/advanced, so budgets run $40–$200 and 60%
sit below $93). "Share" is the pool-average price factor, which is the
fraction of the no-price arrival rate that still comes:

| Day ticket | rating 0.5 (ref $60) | rating 1.0 (ref $90) |
|---|---|---|
| $30 | 100% | 100% |
| $60 (default) | 77% | 77% |
| $90 | 29% | 43% |
| $120 (2× default) | 12.5% | 15% |
| $150 | 4% | 5% |
| $201 (above every budget) | 0% | 0% |

At the default price and neutral rating the factor is exactly the old
binary budget gate (77%), so default arrivals are unchanged. Doubling
from $60 to $120 cuts arrivals by about 84%. The hard budget cutoff alone
accounts for 68 points of that, because only 24% of guests have a $120
budget. The budget distribution, not the elasticity, dominates the
response to large price moves.

---

## Payment: the day ticket

Revenue is per visit, not per ride (VISION §7). The day ticket is paid
**once, at arrival**, inside `spawnGuest`:

- `World.DayTicketPrice` (dollars; default `DefaultDayTicketPrice` = $60
  in `world.go`) is set by the player from the parking lot or ticket
  office popup, and persisted in the save (`day_ticket`).
- The price is computed before planning so the planner sees the
  post-ticket budget: `RemainingBudget = DailyBudget − price`,
  `Guest.DayTicketPaid = price`.
- Only once the spawn succeeds does the money move: `World.Cash += price`
  and `History.RecordRevenue(price)`. A failed spawn charges nothing.
- Guests holding a valid season pass (`SeasonPassExpiry > SimTime`) pay
  nothing.
- Cable lifts cost nothing per ride. Heli keeps per-ride pricing via
  `Lift.RideFare()` (heli's `TicketPrice`; 0 for every other lift type).
- A guest who buys a season pass later in the same visit has today's day
  ticket credited: they pay `SeasonPassPrice − DayTicketPaid`.

---

## Spawning

`Simulation.spawnGuest(lot, g)` places the guest on the mountain:

1. Set `Pos` to the lot's door cell, `Balance = 1.0`, `Patience = 1.0`,
   `Satisfaction = 0.6`; price the day ticket and set `RemainingBudget`.
2. Append to `w.OnMountain`; flip `g.State = OnMountain`.
3. Call `s.replan(g)` — the L0 planner picks the first lift and
   `onPlanStepStart(ActWalkToLift)` lays a pathfinder route.
4. If the planner returns no plan OR the pathfinder fails, unwind: pop
   from `OnMountain`, call `g.ResetForDeparture()`, return false.
5. On success, credit the day ticket to `Cash` and revenue history, and
   record the arrival.

---

## Departure → rating

The GOAP `ActDepart` action is dispatched by `onPlanStepStart`:

```go
case ai.ActDepart:
    s.Demand.recordDeparture(a, s.SimTime)
    lot.CurrentCars -= 1.0 / float32(GuestsPerCar)
    a.Removed = true   // reaped after the tick loop
```

`recordDeparture(g, simTime)`:

1. **Capture** `g.LastScore = g.Satisfaction` — the final session score.
2. **EMA** `ResortRating += ratingEMAAlpha × (g.Satisfaction − ResortRating)`
   with α = 1/70, giving a ~50-departure half-life. Slow enough that one
   grumpy guest can't tank the score; fast enough that improvements
   (better terrain, shorter queues) show up in arrivals within a session.
3. **Bump** `g.LifetimeVisits`, `g.VisitsThisSeason`, `g.LastVisit`.

Rating therefore reflects *completed* sessions — word-of-mouth from guests
who finished their day — rather than a snapshot of whoever is mid-run.

See **GUESTS.md** for how `Satisfaction` is built up during the session
(terrain-quality drift + discrete event spikes).

---

## HUD surface

The top bar's happiness bar reads `s.Demand.ResortRating` directly
(`internal/scene/scenario.go`). The bootstrap value (0.5) shows a neutral
half-bar on a fresh scenario; the bar drifts up or down as guests depart.

---

## Tunables (in `demand.go`)

| Constant | Default | Effect |
|---|---|---|
| `GuestsPerCar` | 4 | Car count increment per spawn / decrement per departure |
| `demandPollInterval` | 30 s (sim) | How often the per-guest poll pass fires |
| `ratingEMAAlpha` | 1/70 ≈ 0.014 | EMA blend factor; ~50-departure half-life for ResortRating |
| `initialResortRating` | 0.5 | Bootstrap value on fresh sim |
| `avgSessionSec` | 800 | Capacity estimate denominator |
| `seasonDaysApprox` | 186 | Divisor for per-day visit rates |
| `world.DefaultDayTicketPrice` | $60 | Starting day ticket; player-adjustable via `World.DayTicketPrice` |
| `world.DayTicketReferencePrice` | $60 | No-penalty day-ticket price at rating 0.5 |
| `world.DayTicketRatingPremium` | 1.0 | How far rating shifts the reference (±50% across rating 0..1) |
| `world.DayTicketElasticity` | 0.5 | Exponent on the price factor; lower = guests tolerate prices closer to budget |

---

## What's not in here (yet)

- **Time-of-day / weather modulation.** The poll fires at a flat rate.
  A clock + weather system would shape the curve (morning peak, storm
  penalty, etc.).
- **Per-lot draw weighting.** New guests pick a uniform-random lot.
  Occupancy, distance to lifts, or pricing could weight the draw.
- **Richer `terrainMatch`.** Binary today. A trail-aware fraction of
  matching lifts lands once trails are first-class entities.
- **Loyalty / repeat-visit hysteresis.** Guests could remember their
  last `LastScore` and be more or less likely to return based on it.
- **Rating display beyond the happiness bar.** A dedicated info window
  would surface recent depart scores and the rating trend.

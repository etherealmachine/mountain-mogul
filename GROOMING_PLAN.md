# Grooming System Redesign Plan

## Goal

Replace the per-shed `Cats int` field with a global cat pool and an intelligent trail-assignment algorithm so players make meaningful placement and fleet-sizing decisions.

Player decisions:
- How many sheds do I build, and where?
- How many cats do I buy?
- Which shed does each cat live in?
- Which cats run active vs. standby?

---

## Economy Model

### Purchase

One-time cost per cat, expensive enough that buying many is a real commitment. Suggested:

| Item | Cost |
|---|---|
| Shed (first cat included) | $30,000 |
| Additional cat | $25,000 |

The first cat shipped with a shed is free in the sense that it's bundled into `ShedCost`; every cat beyond that costs $25,000 regardless of which shed it ends up in.

### Running Costs

Charged per in-game day (the economy tick). Players can park cats in standby to save money at the cost of trail quality.

| Status | Daily cost per cat |
|---|---|
| Active | $500 / day |
| Standby | $75 / day |

These constants live in `world/snowcat.go` alongside the other economy values.

---

## Data Model Changes

### `world/snowcat.go`

```go
const (
    CatPurchasePrice   = 25_000
    CatActiveCostDay   = 500
    CatStandbyCostDay  = 75
    // existing constants stay
)

type CatStatus uint8
const (
    CatActive  CatStatus = 0  // grooming normally
    CatStandby CatStatus = 1  // parked at shed, cheaper maintenance
)
```

Add `Status CatStatus` to `Snowcat`. Standby cats still have a `ShedID`; they just don't leave the shed.

Remove the derived constants `MaxCatsPerShed` and `CatCost` (they're replaced by the above). There is no longer a hard cap per shed.

### `world/building.go`

Remove `Building.Cats int` entirely. The shed no longer owns a count; it is just a home base. Cat count per shed is derived live as `len(w.CatsOwnedBy(shedID))`.

### `world/world.go`

Add `OwnedCats int` to `World`. This is the authoritative count of purchased cats. Invariant:

```
len(w.Snowcats) == w.OwnedCats
```

(Every purchased cat is always in the world and always assigned to exactly one shed.)

### Save format

- Remove `cats` field from shed save entries.
- Add `status` field to snowcat save entries (0 = active, 1 = standby).
- Add `ownedCats` to world save root.

---

## UI (Shed Panel)

The shed info panel (wherever that currently lives in `scene/`) shows:

```
[ Snowcat Fleet ]
  3 cats assigned  (2 active, 1 standby)

  Cat #1  [Active]   [→ Standby]
  Cat #2  [Active]   [→ Standby]
  Cat #3  [Standby]  [→ Active]

  [+ Buy cat  $25,000]   [- Release cat]

[ Coverage ]
  Assigned trails: Panorama, North Bowl
  Trail cells: 340   Cats needed (est.): 2
  ✓ Fleet adequate
```

**"Buy cat"** — deducts $25,000, creates a new `Snowcat` assigned to this shed with status Active, increments `OwnedCats`, triggers a global section reassignment.

**"Release cat"** — removes the most recently standby'd cat (or any idle cat) from this shed. Decrements `OwnedCats` and despawns the cat. No refund — this is not a sell; it's like letting the lease lapse.

**Standby toggle** — switches `cat.Status`. Standby cats park at the shed door and stop grooming. Section reassignment runs so the remaining active cats absorb the work.

**Moving cats between sheds** — not a direct "move" action; instead, the player releases from shed A (standby → release) and buys at shed B. The purchase cost is waived when the cat is re-homed this way — it's the same cat. Implementation: the "Release" button doesn't despawn when OwnedCats > assigned count; it creates a floating "unassigned" cat that the player can then claim at another shed. Simplest path for now: skip this and just show the current assignment per shed; players can buy/release to rebalance.

---

## Trail Assignment Algorithm

### When to run

Reassignment is global and triggered by any fleet or trail configuration change:
- Cat purchased or released
- Cat status toggled (active ↔ standby)
- Shed placed or demolished
- Trail's `Groomed` flag toggled
- Trail cells change (player edits trail)

The current `assignMissingSections()` incremental approach is replaced by a full `reassignAllSections(w)` call on each of these events. It is **not** called every tick.

### Algorithm: Global Voronoi column partition

Sheds are home bases, not assignment units. All active cats across all sheds compete directly for trail columns. There is no trail-to-shed step.

**Single pass:**

1. Collect every `(trailID, xCol)` pair from all groomed trails.
2. For each pair, compute the distance from that column's centroid to the door of every shed that has at least one active cat.
3. Assign the column to the nearest such shed. (Ties broken by shed ID for determinism.)
4. Within each shed, sort its assigned `(trailID, xCol)` pairs — first by trailID, then by xCol — and divide them evenly across the shed's active cats by index range.

That's it. No capacity budget, no overflow pass, no two-phase logic.

**Why this works:**
- A trail near shed A gets its columns claimed by shed A's cats naturally.
- A long trail sitting between two sheds gets split down the middle, both sheds covering their half — better than forcing whole-trail assignment.
- Player shed placement directly controls coverage: put a shed near the back bowls and those cats own those columns.

**Section data structure:**

`SectionTrailID uint64` and `SectionXs []int` are replaced by a single slice:

```go
type CatColumn struct {
    TrailID uint64
    X       int
}

// In Snowcat:
Section []CatColumn  // assigned (trail, x-col) pairs, sorted trailID then X
```

`SliceXs` in the active route state similarly becomes `[]CatColumn`. `SliceIdx` indexes into it as before.

**Edge cases:**
- Shed with zero active cats: its columns are uncovered (nearest shed with cats takes them on next reassignment event — if the shed is the only one, columns are ungroomed until a cat is activated).
- More cats than columns: trailing cats get an empty section and sit idle at the shed.
- Trail removed while cat is mid-pass: existing "clear if trail gone or ungroomed" guard in `tickSnowcats` handles it; full reassignment fires on the config-change event.

---

## Capacity Math

The "Cats needed (est.)" display is computed from observable quantities:

```
groomPassTime(trail, nCats) = (len(trail.Cells) / nCats) * (CellSize / SnowcatSpeed)
                            = cells/nCats * 1.25  seconds

wearPeriod(trail) = time for avg grooming to drop from 1.0 to sectionGroomThreshold (0.5)
                  = 0.5 / observedWearRate(trail)

catsNeeded(trail) = ceil(groomPassTime(trail, 1) / wearPeriod(trail))
                  = ceil(1.25 * cells / wearPeriod)
```

`observedWearRate` is the recent per-cell grooming decay rate measured in `tickSnowcats` (or estimated from skier traffic density). During early development, use a fixed estimate tied to guest count:

```
estimatedWearRate = groomingWearRate * (activeGuests / trailCells) * trafficFactor
```

The shed panel shows total `catsNeeded` across all columns currently assigned to that shed's cats vs. the shed's active cat count, with a simple ✓ / ⚠ indicator.

A global "fleet summary" overlay (future) can show per-difficulty averages: "avg green: 180 cells, needs 1 cat; avg blue: 320 cells, needs 2 cats."

---

## Implementation Sequence

1. **Data model** — add `CatStatus` to `Snowcat`, remove `Building.Cats`, add `OwnedCats` to `World`. Update save/load. Update all callers of `Building.Cats` (UI, sim, economy tick).

2. **Economy tick** — charge `CatActiveCostDay` / `CatStandbyCostDay` per cat per day. Replace the old per-shed cat cost math.

3. **Buy / Release UI** — wire up the shed panel buttons. Buy path: deduct money, `SpawnSnowcat`, set `Status = Active`, call `reassignAllSections`. Release path: set standby first tick, then remove after confirmation.

4. **Global reassignment** — implement `reassignAllSections(w)` replacing `assignMissingSections`. Extend `SectionTrailID` → `SectionTrailIDs []uint64`. Trigger on config events.

5. **Multi-trail section routing** — update `advanceCat` and `sectionAvgGrooming` to handle sections that span multiple trails.

6. **Capacity display** — compute `catsNeeded` and show in shed panel alongside the standby/active controls.

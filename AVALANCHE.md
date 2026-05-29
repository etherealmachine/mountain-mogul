# Avalanche System

## Overview

Avalanches fire as a daily weather effect. After heavy snowfall or rain, steep cells with unstable snow are checked for release. Released snow flows downhill as a hazard corridor; skiers caught in it are injured and require patrol rescue.

---

## Instability score

Each cell is scored once per triggering weather day:

```
slope = |GradientAt(x, z)|   // rise/run, dimensionless (consistent with snowgen thresholds)
if slope < 0.55 (≈29°): never releases

slopeExcess = clamp((slope − 0.55) / 0.85, 0, 1)   // 0 at 29°, 1 at ~60°

kindMult:
  WindSlab, Crust        → 1.5   (slab surface — highest release risk)
  FrozenGranular         → 1.2
  Powder, Cement         → 1.0
  Slush, Corn            → 0.8   (wet-slide risk)
  PackedPowder, Boilerplate, Base → 0.2 (stable)

treeAnchor = TreeDensity × 0.4   // dense trees prevent release

instability = (TotalSWE / 0.30) × slopeExcess × kindMult × (1 − treeAnchor)
```

A cell releases when `instability ≥ 1.0`, subject to a stochastic roll of `clamp(instability − 1.0, 0, 1) × 0.6` so the same conditions don't produce identical events every storm.

---

## Trigger

`checkAvalanches` is called from `applyDailyWeather` when:
- A snowfall day deposited `AccumSWE > 0.04 m` (4 cm SWE), or
- A rain event fired.

---

## Release and propagation

On release, a cell:
1. Clears its `Top` layer and reduces `Base` SWE by 60%.
2. Seeds a BFS flowing downhill for up to 30 hops.
3. Each hop: the steepest-downhill neighbour (via `GradientAt`) receives the transported SWE as `KindBase` debris added to its `Base`; the cell is marked in `avalancheHazard`.
4. Transported SWE × 0.85 per hop (some snow settles at each step).

BFS stops at a cell when:
- Slope drops below 0.25 (flat runout)
- `Passable == false` (building or lift endpoint)
- `TreeDensity > 0.7`
- Hop limit reached

---

## Hazard map

`avalancheHazard [][]float32` lives on `Simulation` (not saved, not on `Cell`). All cells in the release and run-out path are set to 1.0. Decays at −0.04/sim-second in `subTick`, reaching zero in ~25 sim-seconds (several in-game hours).

---

## Skier impact

Each tick in `tickSkier`, before the balance update:

```
if avalancheHazard[xi][zi] > 0.3:
    a.Balance = −1.0       // guaranteed fall
    injuryChance = 0.7     // high — avalanche is very dangerous
```

The existing fall/injury path then handles the rest: patrol dispatch, rescue, satisfaction penalty.

---

## Future work

- Player-side mitigation: explosive triggering, run closures, avalanche barriers.
- Avalanche forecast overlay on the map.
- Separate avalanche event type in guest history for demand/reputation modeling.

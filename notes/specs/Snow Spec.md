---
title: Snow Spec
kind: spec
---

# Snow System

The terrain is two stacked structures — ground (rock/dirt) and a snow column on top whose depth and character vary across the map and evolve over time. The snow column is a **stack of layers**, one per depositional event or major transition. Most of the game reads the *surface* (ground + total snow depth); only a few things care about what is buried beneath.

## Elevation contract

```
SurfaceElevation = GroundElevation + SnowDepth
```

| Reader | Reads |
|--------|-------|
| Visible terrain mesh | Surface |
| Skier physics integrator | Surface |
| Skier perception (`NormalAt`) | Surface |
| Skier targets (lift base, lodge door) | Surface |
| Lift cable, towers, stations | Surface |
| Lodge mesh | Surface |
| Pathfinder (walking agents) | Surface |
| Decorative objects (trees, rocks) | Ground † |
| Editor raise/lower brush | Ground |
| Apron pass (lift, lodge) | Ground ‡ |
| Heightmap import | Ground |

† Trees anchor at `min(SnowDepth, 1.5 m)` above ground so deep snow visually buries the trunk without fully hiding the tree.

‡ The apron raises ground to form a graded earthwork pad, matching how real lift stations sit on built-up benches.

---

## Snow kinds

Seven named kinds are the sole discriminator for density, physics, and rendering behaviour. Continuous `Packed` and `Ice` scalars are gone; each Kind carries fixed constants instead.

| Kind | Density | Base friction | Edge grip | Shader packed | Shader ice |
|------|---------|---------------|-----------|---------------|------------|
| Powder | 0.15 | +80 % drag | −50 % | 0.05 | 0.00 |
| Packed Powder | 0.50 | baseline | baseline | 0.85 | 0.00 |
| Cement | 0.65 | +30 % drag | −10 % | 0.55 | 0.00 |
| Wind Slab | 0.55 | −10 % (fast) | −20 % | 0.50 | 0.10 |
| Crust | 0.60 | −20 % (fast) | −50 % | 0.65 | 0.40 |
| Boilerplate | 0.90 | −50 % (very fast) | −85 % | 0.95 | 0.90 |
| Slush/Corn | 0.55 | +40 % drag | −10 % | 0.45 | 0.00 |
| Debris | 0.65 | +20 % drag | −25 % | 0.20 | 0.45 |

"Shader packed" and "Shader ice" are written into the `aSnow` vertex attribute `(Grooming, Packed, Ice, MogulSize)` via `KindShaderPacked(k)` and `KindShaderIce(k)`. The terrain shader is unchanged — it still reads those two slots, but they are now kind-derived constants rather than per-layer scalars.

`Grooming` modifies the baseline regardless of Kind:
- Base friction × (1 − 0.30 × Grooming)
- Edge grip × (1 + 0.20 × Grooming)

---

## Two-layer model

Each cell stores a `Top` layer and a `Base` SWE accumulation:

```go
type SnowLayer struct {
    Accumulation float32  // metres SWE; conserved as kind transitions
    Kind         SnowKind
}

// On Cell:
Base float32   // consolidated season-base SWE (metres); always KindBase density
Top  SnowLayer // active surface; weather, grooming, and skiing act here
```

`Base` has no `Kind` — it is always `KindBase` (compacted season base). `Top.Kind` varies with weather, traffic, and grooming. Either may be zero (bare ground = both zero). **Invariant**: if any snow exists, `Top.Accumulation > 0` (Base is only ever promoted to a `KindBase` Top during melt, never left exposed as a raw float).

**Semantics:**
- `Top` is always the active surface. Weather transitions, grooming, and skier traffic modify `Top.Kind`.
- `Base` is the accumulated season foundation, always `KindBase`. Snow guns add to it directly.
- When a new storm's kind differs from `Top.Kind`, the current `Top.Accumulation` folds into `Base` (SWE-conserving), and a fresh `Top` begins.
- When `Top` melts away, `Base` is promoted to a `KindBase` `Top` (clearing Grooming and MogulSize).

### Cell-level surface modifiers

Three scalars live on `Cell` rather than in any layer because they describe the current surface state, not a depositional event:

| Field | Range | Meaning | Default |
|-------|-------|---------|---------|
| `Grooming` | 0..1 | Recency/intensity of snowcat passes | 0 |
| `MogulSize` | 0..1 | Mogul amplitude | 0 |
| `SkierTraffic` | float32 | Accumulated skier passes; drives kind transitions | 0 |

### Derived surface properties

| Method | Derived from |
|--------|--------------|
| `TotalSWE()` | `Base.Accumulation + Top.Accumulation` |
| `VisibleSnowDepth()` | Sum of `Accumulation / KindDensity(Kind)` for each non-zero layer |
| `SurfacePacked()` | `KindShaderPacked(TopLayer().Kind)` |
| `SurfaceIce()` | `KindShaderIce(TopLayer().Kind)` |
| `SurfaceElevation()` | `GroundElevation + VisibleSnowDepth()` |
| `SnowAt(x, z)` | `(depth, grooming, kind, mogulSize)` — one-call surface query |

---

## Weather-driven transitions

See [[Weather Spec]] for the full daily rollover sequence. Snow-relevant effects:

### Snowfall — update Top (or fold into Base)

The new storm's Kind depends on temperature:

| Storm TempC | Kind |
|-------------|------|
| < −5 °C | Powder |
| −5 to −1 °C | Packed Powder |
| −1 to 0 °C (clamped) | Cement |

If `Top.Kind` already matches the incoming kind, `Top.Accumulation` grows in place — consecutive powder storms deepen one powder layer. Otherwise the current `Top` folds into `Base` (Base absorbs the SWE; if Base was empty it inherits Top's kind) and a new `Top` begins.

Grooming is diluted: `Grooming *= 1 − min(1, accumSWE / 0.02)`. Two centimetres SWE fully buries any groomed surface.

### Non-precipitation — transition matrix

The **top layer's Kind** changes based on a weather event. The layer below is untouched (unless buried freeze fires — see below).

| Current Kind | Rain | Cold Clear | Warm Clear | Wind |
|--------------|------|------------|------------|------|
| Powder | Cement | Powder | Packed Powder | Wind Slab |
| Packed Powder | Cement | Crust | Slush/Corn | Wind Slab |
| Cement | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Wind Slab | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Crust | Cement | Boilerplate | Slush/Corn | Wind Slab |
| Boilerplate | Cement | Boilerplate | Slush/Corn | Boilerplate |
| Slush/Corn | Slush/Corn | Boilerplate | Slush/Corn | Wind Slab |

Key patterns:
- **Rain** saturates everything → Cement. Slush/Corn stays (already saturated).
- **Cold Clear** hardens → Crust or Boilerplate.
- **Warm Clear** softens → Slush/Corn.
- **Wind** consolidates → Wind Slab. Boilerplate is too hard to reorganise.
- **Overcast** (no wind) → no change.

### Melt

Melt is an enhanced temperature-index model (`sim/snowmelt.go`), run once per clock hour (`Simulation.meltHour`) on non-snow days. Each hour, every cell loses:

```
meltSWE = (0.0015 + 0.0035 × sunExposure) m/°C·day × max(0, T)/24  +  rain share of 0.00025 m/mm × RainMM
```

- **T** is the hour's air temperature from the daily curve (low at sunrise, high at 14:30), lapse-adjusted for the cell's elevation, so a −3 °C clear day peaking at +2 °C still melts a little in the afternoon — and only then.
- **sunExposure** is the current clear-sky direct beam on the cell's slope and aspect (the sun's real position at 45°N, air-mass attenuated), normalised so a flat cell at the equinox averages 1 over the day, and cut by up to 85% under cloud. It is 0 at night, and 0 while surrounding terrain hides the sun: `world.HorizonMap` stores each cell's horizon angle in 16 compass directions (rays marched up to 3 km), so a bowl under a ridge melts at the shade rate on low midwinter mornings. Trees and buildings don't cast shade.
- Cells that melt mark the snow mesh dirty, so warm afternoons rebuild terrain every clock hour.
- The resulting melt factor spans ~1.5 mm/°C·day on a December north face to ~8 mm on an April south face, matching field values for seasonal snow.

Typical outcome over a simulated season: a resort that opens by Dec 10 melts out before January about 1% of the time on flat ground (7% on steep south faces, 0% on north faces), and by mid-April south faces hold a third of the north faces' snow. When `Top` is fully melted, `Base` is promoted to `Top` (clearing Grooming and MogulSize), and any remaining melt budget continues consuming the promoted layer. When both are exhausted, bare ground is exposed.

---

## Skier traffic — kind transitions

Each active-skiing tick accumulates `SkierTraffic` on the cell underfoot. When the counter crosses a per-kind threshold, the top layer transitions and the counter resets:

| Current Kind | Threshold | Transitions to |
|--------------|-----------|----------------|
| Powder | ~40 passes | Packed Powder |
| Wind Slab | ~60 passes | Packed Powder |
| Crust | ~20 passes | Packed Powder |
| Packed Powder, Cement, Boilerplate, Slush/Corn | — | (no traffic transition) |

`SkierTraffic` decays 15 % per in-game day (~4-day half-life) so untrafficked runs reset between busy periods.

`Grooming` decays at −0.02/s from skier passes (unchanged). `MogulSize` grows +0.005·(1−Grooming)/s when `VisibleSnowDepth > 0.3 m` (unchanged).

---

## Snowcat grooming

When a snowcat grooms a cell (`sim/snowcats.go`):

- Top layer Kind → **Packed Powder**
- `Grooming` → 1.0
- `SkierTraffic` → 0

Grooming sets Kind rather than raising a Packed scalar. A freshly groomed run is definitively Packed Powder regardless of what Kind it was before.

---

## Apron pass

When a lift or lodge is placed, a one-shot apron pass runs over the structure's footprint:

1. Raises `GroundElevation` toward a target with smoothstep falloff at the edges.
2. Clamps snow depth to ~5 cm (thin packed pad).
3. Sets top layer Kind → **Packed Powder**, weight-scaled by the falloff.
4. Zeros `MogulSize` proportionally.
5. Ramps the ground beyond the pad onto an embankment no steeper than
   `embankmentGrade` (0.2, about 11°, the beginners' comfort slope) for
   up to `embankmentReach` (60 m), easing back to natural over the last
   15 m. Lift and building aprons only fill. Painted pads (parking lots,
   lodges) cut and fill. Other structures' graded cells are left alone.
   Without this, a raised pad on a slope left a wall that guests skied off.

The apron is a terrain edit; it is serialised into the save and not re-applied on load.
Saves graded before embankments existed can be repaired with the `regrade`
debug-console command, which ramps the ground around every lift station and
painted pad at their current heights.

---

## Rendering

The terrain vertex carries `aSnow = (Grooming, Packed, Ice, MogulSize)` at attribute 4. `Packed` and `Ice` are derived from the top layer's Kind via `SurfacePacked()` and `SurfaceIce()`. The terrain shader (`terrain.frag`) is unchanged and reads these slots as before:

- **Grooming** — corduroy stripes + cool tint.
- **Packed** — blue tint mix.
- **MogulSize** — brightness modulation via value-noise (bump simulation without geometry displacement).
- **Ice** — specular lobe boost + silver-blue tint.

---

## Save format

`CellData` in `internal/save/format.go`:

```
e   → GroundElevation
ls  → []LayerData (0–2 entries: [Base, Top] if present)
gr  → Grooming
mg  → MogulSize
st  → SkierTraffic
td  → TreeDensity
```

Each `LayerData`:
```
a   → Accumulation (SWE metres)
k   → SnowKind uint8 (omitempty; 0 = Powder if absent)
```

`ls` is written with 0 entries (bare ground), 1 entry (Top only), or 2 entries (Base then Top). The Base entry always has `k = KindBase`; its kind is not used on load. Old saves with more than 2 entries load the last two as Base and Top, discarding older history.

---

## Surface detail texture

A 1 m-resolution RGBA8 texture mirroring the terrain, written by the simulation and sampled in `terrain.frag` for sub-cell features the 5 m mesh can't carry: skier tracks, tree wells, groom-edge sharpening.

`world.SurfaceDetail` lives on `Terrain`:

```go
type SurfaceDetail struct {
    PxWidth, PxHeight int
    Pixels            []uint8  // flat RGBA8, row-major
    Dirty             bool
    DirtyBox          image.Rectangle
}
```

| Channel | Meaning | Written by | Persistence |
|---------|---------|------------|-------------|
| R | Track intensity | sim (per skiing tick) | Decays in sim time |
| G | Tree-well depth | world (on tree edit) | Persistent until edit |
| B | Groom-edge mask | derived from `Grooming` | Recomputed on dirty |
| A | Reserved | — | — |

Not saved. Fully re-derivable on load: G stamped from saved `TreeDensity`, B recomputed from saved `Grooming`, R resets to zero.

---

## Physics

`effectiveFriction` in `sim/skiing.go` applies Kind multipliers to the base/edge friction coefficients used by the integrator. For Powder, an additional depth-gated drag kicks in when `VisibleSnowDepth > 0.5 m`. `Grooming` and `MogulSize` modifiers apply on top of Kind, unchanged from before.

---

## Avalanche simulation

Avalanches fire as a daily weather effect. After heavy snowfall or rain, steep cells with unstable snow are checked for release. Released snow flows downhill as a hazard corridor; skiers caught in it are injured and require patrol rescue.

### Instability score

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

### Trigger

`checkAvalanches` is called from `applyDailyWeather` when:
- A snowfall day deposited `AccumSWE > 0.04 m` (4 cm SWE), or
- A rain event fired.

### Release and propagation

On release, a cell:
1. Clears its `Top` layer and reduces `Base` SWE by 60%.
2. Enqueues a propagating chain (`avyChain`) carrying a multi-cell spreading front. Advances at **2 cells/wall-second (~10 m/s)** via `tickAvalancheChains`.
3. Each tick, every item in the front fans out to qualifying forward neighbours. Neighbours are weighted by `dot(travelDir, offset) × effectiveSlope`, where `effectiveSlope = slopeToNeighbour + momentum × 0.1`. SWE splits proportionally — the primary downhill cell gets the most; lateral cells get smaller shares. This makes the avalanche widen as it descends.
4. Transported SWE × 0.85 per hop (15% settles as debris at each reached cell), distributed across all spread children.
5. Each child item carries **momentum** that builds on steep terrain and drains on flat terrain (`avyMomentumGain = 2.0`, `avyMomentumMax = 4.0`). Momentum carries the chain through flat runout zones and short uphill bumps rather than stopping on a dime.

An item stops propagating when:
- All forward-hemisphere neighbours have `effectiveSlope ≤ 0` (flat/uphill and no momentum)
- `Passable == false` (building or lift endpoint)
- `TreeDensity > 0.7`
- Hop limit (30) reached

A 30-hop avalanche takes ~15 real seconds to fully propagate.

### Hazard map

`avalancheHazard [][]float32` lives on `Simulation` (not saved, not on `Cell`). The source cell is set to 1.0 immediately; run-out cells are set to 1.0 as the animated front reaches them. Decays at −0.04/sim-second in `subTick`, reaching zero in ~25 sim-seconds (several in-game hours).

### Skier impact

Each tick in `tickSkier`, before the balance update:

```
if avalancheHazard[xi][zi] > 0.3:
    a.Balance = −1.0       // guaranteed fall
    injuryChance = 0.7     // high — avalanche is very dangerous
```

The existing fall/injury path then handles the rest: patrol dispatch, rescue, satisfaction penalty.

---

## Future work

- **Glade tolerance trait** — per-skier willingness to enter trees; currently all skiers avoid trees equally.
- **Weather-driven demand** — bad weather suppressing arrivals or guest satisfaction.
- **Wind field** — daily wind direction as a simulation variable rather than a static scenario parameter.
- **Lift cable clearance** — cables currently sit at `Surface + CableHeight`; physically they should clear ground regardless of snow depth.
- **Avalanche mitigation** — explosive triggering, run closures, avalanche barriers.
- **Avalanche forecast overlay** — show predicted risk zones on the map before triggering.
- **Avalanche event type** — separate entry in guest history for demand/reputation modeling.

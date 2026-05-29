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

See WEATHER.md for the full daily rollover sequence. Snow-relevant effects:

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

On days with locally positive temperature (after lapse-rate adjustment), `Top` loses SWE:

```
meltSWE = 0.02 m/°C/day × effectiveTempC
```

Rain days add `0.001 m SWE / mm` of rainfall. When `Top` is fully melted, `Base` is promoted to `Top` (clearing Grooming and MogulSize), and any remaining melt budget continues consuming the promoted layer. When both are exhausted, bare ground is exposed.

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

The apron is a terrain edit; it is serialised into the save and not re-applied on load.

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

## Future work

- **Glade tolerance trait** — per-skier willingness to enter trees; currently all skiers avoid trees equally.
- **Weather-driven demand** — bad weather suppressing arrivals or guest satisfaction.
- **Wind field** — daily wind direction as a simulation variable rather than a static scenario parameter.
- **Lift cable clearance** — cables currently sit at `Surface + CableHeight`; physically they should clear ground regardless of snow depth.
- **Avalanche simulation** — the two-layer model directly supports slab instability: `Top` (WindSlab/Crust with significant SWE) over `Base` (Boilerplate/Crust) on slope angles above ~35° constitutes an unstable slab. No layer indexing required.

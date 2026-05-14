# Snow system

The terrain is two stacked layers — ground (rock/dirt) and a snow layer on top
whose depth and state vary across the map and evolve over time. Most of the
game reads the *surface* (ground + snow); only a few things care about the
ground beneath. Snow type is represented as independent per-cell scalars, not
a discrete enum, so transitions and gradients are continuous and "type" labels
(corduroy, powder, moguls, ice) are emergent rather than authored.

## Elevation contract

```
SurfaceElevation = GroundElevation + SnowDepth
```

| Reader                                       | Reads    |
| -------------------------------------------- | -------- |
| Visible terrain mesh (`buildTerrainVerts`)   | Surface  |
| Skier physics — `apply` integrator           | Surface  |
| Skier perception — `NormalAt`                | Surface  |
| Skier targets — lift base, lodge door        | Surface  |
| Lift cable, lift towers, lift station meshes | Surface  |
| Lodge mesh                                   | Surface  |
| Pathfinder agent position during walking     | Surface  |
| Decorative objects (trees, rocks, stumps)    | Ground † |
| Editor "raise/lower terrain" brush           | Ground   |
| Apron pass (lift, lodge)                     | Ground ‡ |
| Real-world heightmap import                  | Ground   |

† Trees and decorative objects anchor at `min(SnowDepth, 1.5 m)` above ground.
The terrain mesh, which sits at `Ground + SnowDepth`, occludes the buried
portion of the trunk via the depth buffer. Capping the anchor at 1.5 m means
deep snow buries more of the trunk visually while still leaving most of the
tree visible.

‡ The apron raises ground to make a graded earthwork pad — same convention as
real lift stations, which sit on built-up benches rather than notched into the
hillside.

## Snow-state scalars

Each cell stores five floats describing the snow on top of it. All are in
`[0, 1]` except `SnowDepth`, which is in metres. Defaults from
`world.NewTerrain` seed loose, lightly-packed snow — close to fresh
powder — so skier traffic and grooming have full compression range:

| Field       | Range  | Meaning                              | Default |
| ----------- | ------ | ------------------------------------ | ------- |
| `SnowDepth` | metres | Thickness of the snow layer          | 2.0     |
| `Grooming`  | 0..1   | Recency/intensity of cat-track passes | 0       |
| `Packed`    | 0..1   | Column density (powder → bulletproof) | 0.2     |
| `Ice`       | 0..1   | Ice fraction at the surface          | 0       |
| `MogulSize` | 0..1   | Mogul amplitude                      | 0       |

**Emergent "types":**

- *Corduroy:* `Grooming` high, `MogulSize` low, `Packed` = 1 (fresh groom).
- *Powder:* `SnowDepth` high, `Packed` low.
- *Moguls:* `Grooming` low + skier traffic over time → `MogulSize` rises.
- *Boilerplate ice:* `Ice` high + `SnowDepth` low + freeze cycle.

`SnowDepth` and `Packed` are coupled by snow-water-equivalent conservation
(see *Snow-water-equivalent coupling* below): as the column packs, the
surface drops proportionally. Traffic packing, depth compression, and
mogul formation are all live; freeze-thaw / precipitation are not.

## Apron pass

When a lift or a lodge is placed, an apron pass runs once over the cells
within a structure-specific footprint. The apron:

1. Raises `GroundElevation` toward a target (station footing + buildup),
   raise-only, with smoothstep falloff on the outer edges. Lifts get a
   40 × 24 m rectangle aligned with the cable axis; lodges get a 24 × 24 m
   square.
2. Clamps `SnowDepth` to ~5 cm (a thin packed pad). This is what the
   renderer keys off to suppress vertex jitter — thin snow → smooth-read
   terrain.
3. Raises `Packed` to the falloff weight (full inside, fading to zero at
   the edge). Aprons are foot-tracked and machine-compacted, so they read
   as packed flat snow — not corduroyed. `Grooming` is deliberately NOT
   raised; if the player grooms over the apron later, that's their choice.
4. Zeros `MogulSize` and `Ice` proportionally to the weight.

The apron is a one-shot terrain edit at placement time. Save loading does
*not* re-apply it — the apron-edited cell state is what's serialised, so
loading a save reconstructs the apron faithfully without re-mutating.

Both pass through a shared `buildStationApron` helper in `internal/scene/
scenario.go`. Adding a new structure type means: add a placement-effects
function that picks footprint + axis + buildup and calls the helper.

## Skier traffic wear

`internal/sim/skiing.go::wearSnowUnderfoot` runs once per skier per tick
on the cell beneath the agent. It mutates four scalars at rates tuned
to the average per-pass exposure (~0.5 s in a 5 m cell at 10 m/s):

| Field       | Rate          | Effect                                   |
| ----------- | ------------- | ---------------------------------------- |
| `Grooming`  | −0.02/s       | Skis cut up the corduroy; ~100 passes wipe out fresh grooming |
| `Packed`    | +0.05/s       | Boots and edges compact the column; ~40 passes saturate from 0.5 |
| `SnowDepth` | coupled       | Column compresses as it packs (see *SWE coupling* below) |
| `MogulSize` | +0.005·(1−Grooming)/s | Ungroomed cells slowly mogul up; corduroy resists bumps |

Mogul growth is gated on `SnowDepth > 0.3 m` so aprons (clamped to
~5 cm) and scraped patches don't grow geometry-less bumps. Fallen,
walking, queueing, and on-lift agents don't run this — only active
skiing in `tickSkier`.

The wear loop sets `Terrain.SnowDirty = true`; the renderer rebuilds
the whole mesh at most once per frame regardless of how many cells
changed. The snowcat fleet refreshes `Grooming` back to 1.0 (and
compresses to `Packed = 1.0`) as cats arrive at cells, so a heavily-
trafficked piste reaches a dynamic equilibrium between wear and
grooming throughput.

### Snow-water-equivalent coupling

`SnowDepth` and `Packed` are linked by a linear density model:

```
density(Packed) = 0.15 + 0.85 × Packed
```

The 0.15→1.0 ratio (~6.7×) is exaggerated vs. real snow (real ratio is
closer to 5–10× from fluff to corduroy, but real default-state snow is
much firmer than our default Packed=0.2). The numbers are picked so the
step at a groomed-lane boundary is readable in a near-top-down view,
not just from a side angle.

When `Packed` changes — either gradually under skier traffic or in one
step under a snowcat — `SnowDepth` is scaled to conserve SWE:

```
SnowDepth_new = SnowDepth_old × density(Packed_old) / density(Packed_new)
```

Concrete effects:

- A fresh-powder cell (`Packed = 0`, 2 m deep) groomed by a cat in one
  pass settles to ~0.30 m of corduroy. Compression ratio = 0.15 / 1.0.
- A default cell (`Packed = 0.2`, 2 m) groomed settles to ~0.64 m.
  Ratio = 0.32 / 1.0; the boundary step is ~1.36 m.
- A handful of skier passes on a default-state cell drops it
  noticeably: 4 passes (≈2 s in-cell exposure) takes Packed from 0.2
  to 0.3, density 0.32 to 0.41, depth 2 m to ~1.56 m — a 44 cm
  visible dimple from light traffic alone.

The compression is what the player sees: groomed and well-tracked lanes
sit visibly lower than the adjacent untracked snow shoulder. The
divergence-driven flat-normal lighting in `terrain.frag` accentuates
the step at the boundary so the shadowed shelf reads clearly even
without a side-on camera angle.

Aprons (lift, lodge) write `Packed = 1.0` and `SnowDepth ≈ 0.05 m`
directly as a one-shot structure-placement edit — they bypass the SWE
formula because they represent graded earthwork, not a physical
compression of pre-existing snow.

## Rendering

The terrain mesh's vertex layout carries four snow-state scalars per vertex,
packed as `vec4 aSnow = (Grooming, Packed, Ice, MogulSize)` at attribute 4
(stride 12 floats per vertex). Per-corner sampling: vertex at grid corner
`(x, z)` reads the cell at the same indices (clamped to the visible mesh's
`(W-1)×(H-1)` interior cells), so each cell contributes its snow state to
the four corners of its quad.

`assets/shaders/terrain.frag` consumes them:

- **Grooming** drives a 4-stripes-per-metre corduroy sine oriented along
  the contour (perpendicular to the local fall line). The fall direction
  comes from the heavily filtered per-vertex `smoothNormal` (5-tap
  binomial × 2 passes on the elevation grid, then central differences),
  interpolated per fragment — so the stripes smoothly curve with the
  terrain instead of fragmenting at triangle edges. Faded in over the
  slope range `|N.xz| ∈ [0.05, 0.15]` (~3° to ~9°) so flat aprons don't
  pick up noise-rotated direction from near-zero gradients. Amplitude
  scales with `Grooming`; gated off on cliffs and skirts. A subtle
  value-noise grain and a cool tint also key off `Grooming` to mark
  groomed surfaces beyond the cords themselves.
- **Packed** mixes a bluer cool tint into the base color (~20 % at full).
- **MogulSize** modulates brightness via a two-octave value-noise at ~3 m
  feature size, simulating the highlight/shadow pattern of bumps without
  actually displacing geometry.
- **Ice** boosts the existing sparkle gate, adds a broader specular lobe
  with a silver-blue tint, and dramatically raises overall specular intensity.

Skirt walls and the floor face emit `aSnow = (0, 0, 0, 0)` so none of the
snow effects bleed onto the cliff sides or the underside of the diorama.

## Physics

`internal/sim/skiing.go::effectiveFriction` samples snow state at the
skier's position and modulates the base/edge friction coefficients used by
the integrator. Effects, expressed as multipliers on the corduroy baseline
`(muBase, muEdge)`:

| Field       | Base friction | Edge friction | Notes                                   |
| ----------- | ------------- | ------------- | --------------------------------------- |
| `Grooming`  | × (1 − 0.30g) | × (1 + 0.20g) | Glide + grip — clean carves             |
| `Packed`    | × (1 − 0.10p) | × (1 + 0.10p) | Mild glide + grip                       |
| Powder gate | × (1 + 0.80w) | × (1 − 0.50w) | `w = (1−packed)·clamp(depth/2.5)`; depth>0.5 only |
| `MogulSize` | × (1 + 0.60m) | × (1 + 0.10m) | Uniform bleed (geometric moguls not yet there) |
| `Ice`       | × (1 − 0.50i) | × (1 − 0.85i) | Both tank — the brake angle stops working |

Numerical floors prevent extreme inputs from driving friction to zero.

Skier perception (`NormalAt`) reads the snow surface, so steep snow drifts
deflect the perceived fall line correctly. There are no other snow-aware
hooks in perception or behavior yet — glade tolerance, "powder hunting"
goals, and ice-avoidance traits are roadmap items.

## Save format

Stable JSON tags in `internal/save/format.go` so the on-disk schema is
robust to internal renames:

```
e   → GroundElevation   (formerly Elevation)
s   → SnowDepth
gr  → Grooming
pk  → Packed
ic  → Ice
mg  → MogulSize
td  → TreeDensity
```

The original `e` tag was preserved through the rename so saves that predate
the snow system still load — their `e` field populates `GroundElevation`,
their `s` field populates `SnowDepth`, and the new fields default to zero
(behaving as flat ungroomed snow at whatever depth was saved). New saves
fill in the full set.

## Future work

- **Snow brushes.** Player-facing tools to paint `SnowDepth`, `Grooming`,
  spray snowmaking, etc. The data model supports them; UI is the remaining
  work.
- **Snowfall and freeze cycles.** Each tick: snowfall adds `SnowDepth`
  (typically as low-`Packed`, low-`Ice` accumulation, lifting the surface
  back up); thin-snow + traffic + freeze cycles raise `Ice`. Skier wear
  (Grooming decay, Packed rise, SWE-conserving depth compression, Mogul
  growth) and snowcat grooming (full pack + compression + corduroy) are
  already implemented; precipitation and freeze-thaw are not.
- **Glade tolerance trait.** Per-skier willingness to enter trees;
  currently all skiers avoid trees equally. Already on the roadmap.
- **Lift cable clearance over ground.** Cables currently sit at `Surface +
  CableHeight`. In deep snow the cable lifts proportionally with snow
  depth; physically the cable should clear the *ground* at a constant
  height regardless of snow accumulation, but the visual difference is
  small at typical depths so this is a deferred polish item.

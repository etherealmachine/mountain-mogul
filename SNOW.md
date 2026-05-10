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
`world.NewTerrain` are roughly "fresh dump, moderately packed":

| Field       | Range  | Meaning                              | Default |
| ----------- | ------ | ------------------------------------ | ------- |
| `SnowDepth` | metres | Thickness of the snow layer          | 2.0     |
| `Grooming`  | 0..1   | Recency/intensity of cat-track passes | 0       |
| `Packed`    | 0..1   | Column density (powder → bulletproof) | 0.5     |
| `Ice`       | 0..1   | Ice fraction at the surface          | 0       |
| `MogulSize` | 0..1   | Mogul amplitude                      | 0       |

**Emergent "types":**

- *Corduroy:* `Grooming` high, `MogulSize` low, `Packed` moderate.
- *Powder:* `SnowDepth` high, `Packed` low.
- *Moguls:* `Grooming` low + skier traffic over time → `MogulSize` rises.
- *Boilerplate ice:* `Ice` high + `SnowDepth` low + freeze cycle.

These dynamics (traffic packing, mogul formation, freeze-thaw) are not yet
implemented — currently snow state is set at terrain construction and by the
apron pass, and otherwise stays constant. See *Future work* below.

## Apron pass

When a lift or a lodge is placed, an apron pass runs once over the cells
within a structure-specific footprint. The apron:

1. Raises `GroundElevation` toward a target (station footing + buildup),
   raise-only, with smoothstep falloff on the outer edges. Lifts get a
   40 × 24 m rectangle aligned with the cable axis; lodges get a 24 × 24 m
   square.
2. Sets `Flat` (a render-side hint that suppresses vertex jitter).
3. Clamps `SnowDepth` to ~5 cm (a thin groomed pad).
4. Sets `Grooming` to the falloff weight (full inside, fading to zero at
   the edge).
5. Zeros `MogulSize` and `Ice` proportionally to the weight.

The apron is a one-shot terrain edit at placement time. Save loading does
*not* re-apply it — the apron-edited cell state is what's serialised, so
loading a save reconstructs the apron faithfully without re-mutating.

Both pass through a shared `buildStationApron` helper in `internal/scene/
scenario.go`. Adding a new structure type means: add a placement-effects
function that picks footprint + axis + buildup and calls the helper.

## Rendering

The terrain mesh's vertex layout carries four snow-state scalars per vertex,
packed as `vec4 aSnow = (Grooming, Packed, Ice, MogulSize)` at attribute 4
(stride 12 floats per vertex). Per-corner sampling: vertex at grid corner
`(x, z)` reads the cell at the same indices (clamped to the visible mesh's
`(W-1)×(H-1)` interior cells), so each cell contributes its snow state to
the four corners of its quad.

`assets/shaders/terrain.frag` consumes them:

- **Grooming** drives a 4-stripes-per-metre corduroy sine oriented along the
  contour (perpendicular to the local fall line). Amplitude scales with
  `Grooming`; gated off on cliffs and skirts.
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
f   → Flat
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
- **Snowcat agent.** AI agent that drives along trails raising `Grooming`
  and crushing `MogulSize`. Currently grooming is only applied by the
  apron pass at placement time.
- **Dynamic snow state.** Each tick: snowfall adds `SnowDepth`; skier
  traffic raises `Packed` and slowly grows `MogulSize` in ungroomed cells;
  thin-snow + traffic + freeze cycles raise `Ice`. None of these dynamics
  exist yet — snow state is static between placement events.
- **Glade tolerance trait.** Per-skier willingness to enter trees;
  currently all skiers avoid trees equally. Already on the roadmap.
- **Lift cable clearance over ground.** Cables currently sit at `Surface +
  CableHeight`. In deep snow the cable lifts proportionally with snow
  depth; physically the cable should clear the *ground* at a constant
  height regardless of snow accumulation, but the visual difference is
  small at typical depths so this is a deferred polish item.

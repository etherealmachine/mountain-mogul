# Snow surface detail

A 1 m-resolution RGBA8 texture mirroring the terrain, written by the
simulation and sampled in `terrain.frag`, so we can render sub-cell
features the 5 m mesh can't carry: skier tracks, tree wells, sharper
groomed/ungroomed edges. Complements (not replaces) the existing
FS procedural normal-kick — texture says *where* the special features
are; FBM gives the high-frequency surface noise everywhere else.

Resolution is **5× finer than the terrain cell grid**. For a 60×60
terrain (300 m square) that's 300² pixels × 4 bytes ≈ 360 KB. Scales
linearly; not a concern at any plausible map size.

## Data model

`world.SurfaceDetail` lives on `Terrain`, alongside `Cells`:

```go
type SurfaceDetail struct {
    PxWidth, PxHeight int    // = Width*5, Height*5
    Pixels            []uint8 // flat RGBA8, row-major (px-major)
    Dirty             bool
    DirtyBox          image.Rectangle // px-space; expanded by writers
}
```

Channels:

| ch | meaning           | written by                          | persistence            |
|----|-------------------|-------------------------------------|------------------------|
| R  | track intensity   | sim, per skiing agent per substep   | decays in sim time     |
| G  | tree-well depth   | world, on tree placement / removal  | persistent until edit  |
| B  | groom-edge mask   | derived from `Cells[*].Grooming`    | recomputed on dirty    |
| A  | reserved          | —                                   | —                      |

Not saved. `SurfaceDetail` is fully re-derivable on load:
- G is stamped from the saved `TreeDensity` field.
- B is recomputed from the saved per-cell `Grooming` field.
- R resets to zero — equivalent to "morning after fresh snow," which
  is the right default anyway.

## Lifecycle

### Tree wells (G)
- `Terrain.RestampTreeWells()` iterates cells with `TreeDensity > 0`,
  writes a Gaussian-falloff radial disk (~2 m radius, peak 1.0) into G
  at each tree's world XZ.
- Called from:
  - `Scenario.installWorld` after load,
  - `GenerateTreeCover`,
  - `applyTool` for the Glade / Plant brushes.

### Groom-edge mask (B)
- `Terrain.RecomputeGroomEdges()` single-pass scan over the buffer.
  For each 1 m pixel, look up its parent cell's `Grooming`; if any
  4-neighbour cell differs, write `B = 1 − distToEdge/1m`.
- Cheap — O(map size) — and runs on the same dirty signal as
  `Terrain.SnowDirty`, which is already set whenever grooming changes.

### Skier tracks (R)
- `Terrain.SplatTrack(wx, wz, intensity)` — additive 3×3 disk centred
  on the pixel under the foot position, capped at 255. Marks `Dirty`
  + extends `DirtyBox`.
- Hook in `sim.tickSkier`: per-substep, only when the agent is
  *actually* skiing (`!Fallen && OnLiftID == 0 && !Queued && Speed > minSpeed`).
- **Segment splat**: interpolate along (lastPos → curPos) so fast
  skiers leave continuous lines instead of dotted ones. Cheap — most
  segments are 1-3 pixels long.
- **Decay**: piggybacks on the 30 s `Demand.maybePoll` cadence. One
  linear pass over R multiplying by `0.985` (≈ 30-min half-life in
  sim time). Re-stamps the dirty box to the full texture.
- **Grooming clears tracks**: `tickSnowcats` zeros the R channel
  inside the cat's footprint each tick — real grooming destroys
  tracks.

## GPU mirror

In `render.Renderer`:

```go
SnowSurfaceTex uint32 // RGBA8, GL_LINEAR, GL_CLAMP_TO_EDGE
```

- `Renderer.BuildSnowSurfaceTex(t *world.Terrain)` — allocates,
  initial upload. Called from `installWorld` after
  `BuildTerrainMesh`.
- `Renderer.FlushSnowSurface(t *world.Terrain)` — `glTexSubImage2D`
  over `DirtyBox`, clears `Dirty` and resets the box. Called from
  `Scenario.Update` next to the existing `FlushSnowState` block.

Sampler binds at texture unit 1.

## Shader integration (`terrain.frag`)

New uniforms:

```glsl
uniform sampler2D uSnowSurface;
uniform vec2      uWorldSize; // (Width*5, Height*5) in metres
```

Sample once near the top of `main()`:

```glsl
vec4 surf = texture(uSnowSurface, vWorldPos.xz / uWorldSize);
float track = surf.r;
float well  = surf.g;
float edge  = surf.b;
```

Then, per feature:

### Tracks (R)
- Increase local `packed` toward 1 — track lanes read as compressed.
- Darken `snow` colour by ~3 % proportional to `track`.
- Kick `Nshading` along ∇R (finite-difference sample 4 times around
  the fragment). The gradient direction *is* the track direction;
  no need to store heading separately, and the kick magnitude
  produces the carved-groove silhouette feel.

### Tree wells (G)
- Subtract `0.5 · well` metres from `vSnowDepth` so `snowness` drops
  at the well centre and ground starts showing through at the base.
- Add a downward bump to `Nshading` proportional to ∇G, giving the
  small depression around the trunk.

### Groomed/ungroomed edge (B)
- Sample neighbouring fragments' snow depth to determine which side
  has powder (higher depth) vs. groomed (lower).
- Add a thin bright-white lip on the powder side (the natural
  shoulder where untracked snow piles against the grooming pass).
- Add a slightly darker / cooler line on the groomed side (compacted
  edge).
- Replaces the current soft 2-cell ramp — keeps the geometry as-is
  but makes the edge visually crisper.

## Independent: FBM extension

Replace each single `valueNoise` call in `terrain.frag`'s
powder/mogul kicks with a 3-octave FBM (×1, ×2, ×4 frequency;
amplitudes ×1, ×0.5, ×0.25). ~6 extra `valueNoise()` calls per
fragment. Pure shader change, no Go-side work, ships independently of
the surface-detail texture.

## Implementation order

1. **Infrastructure** — `SurfaceDetail` struct + buffer accessors,
   GPU texture build/flush, shader uniform, debug overlay bit that
   renders each channel as a colour for verification. No effects
   yet.
2. **Tree wells** — easiest sim-side write (one-shot at place time),
   persistent, validates the write→upload→sample loop. Visible
   immediately in the debug overlay; depression visible once the FS
   effect lands.
3. **Groom-edge mask + shader lip** — replaces the soft 2-cell ramp
   with a sharper visual edge. Cell-to-pixel scan is straightforward.
4. **Skier tracks** — splat in `tickSkier`, decay on the 30 s demand
   cadence, clear on cat groom. Most exciting visually; biggest
   hot-path code path so it's last to land.
5. **FBM extension** — independent, can ship at any time.

## Open questions

- **Sim-time vs wall-time decay** — default to sim-time so the
  cadence is consistent with everything else that ages (cat
  routes, snow accumulation, demand polls). Wall-time would keep
  tracks "alive" longer during fast-forward, which is probably
  *not* what we want.
- **Skier dogpile** — segment-splatting between last-tick and
  this-tick positions is the right answer; the alternative
  (single-point splats) produces visible dots at high speed /
  high TimeScale. Cheap to do, in scope for step 4.
- **Cat groom clears tracks** — confirmed yes in the lifecycle above;
  matches the player's expectation that grooming wipes the slate.

## Estimated cost

- Steps 1–4: ~1–2 days (texture pipeline is most of the work; each
  shader effect is small).
- Step 5: half a day, can land any time.

## Future hooks

Channel A is reserved for:
- Ice patches around lift bases / lodge stoops.
- Herringbone marks from skiers walking uphill.
- Footprints from non-skiing agents (lift queue shuffling, etc).
- Snowfall accumulation visual delta (fresh snow brightening a
  region that was tracked-out the day before).

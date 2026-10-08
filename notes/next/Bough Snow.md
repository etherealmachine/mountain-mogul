---
title: Bough Snow
kind: plan
status: planned
---

# Bough Snow

Snow on the trees that builds up through a storm and lingers after it: held for days in cold shade and up high, gone first from sunny and low slopes, shaken off by wind, washed off by rain. Part of [[Trees]], [[Weather]], and [[Rendering]]; the leftover from [[Graphics Base]] step 4. *For* [[Asahidake]] and any storm-heavy map; it's what sells "the morning after a dump".

## Why

Read on 2026-10-08:

- **One number for the whole map, from today's weather.** The scene sets `Renderer.CanopySnow` each frame from today's state (`render.CanopySnowFor`, `internal/render/lighting.go`): 1 on a heavy-snow day, 0.6 on a light-snow day, 0 otherwise. `static.frag` whitens the upper boughs by that amount with per-tree noise.
- **So it switches, it doesn't linger.** At the day rollover after a storm every tree goes bare green at once, while the ground is at its deepest and freshest. Real conifers hold snow for days in cold weather.
- **And it's the same everywhere.** No difference with elevation, aspect, or shade, though the sim already computes all three hourly for the ground's melt (`Simulation.meltHour`: temperature lapsed by elevation, sun exposure by slope and aspect, terrain shadow from the horizon map).
- **Trees are drawn from a static batch** (`DrawWorld` builds one instance per tree), so per-tree data would mean rebuilding the batch whenever the snow changes. A small per-cell texture the shader samples by position avoids that, as the snow surface already does.

## The model

**Canopy load per cell.** Each cell with trees holds the snow on its branches as water equivalent (mm), capped by what branches can carry (interception capacity, a few mm; more where the cover is dense). Cells without trees hold nothing.

**Updated hourly** in `tickHourly`, every hour, not only when it's warm (the ground's melt pass skips cold dry hours and snowy days):

- **Snowfall loads it.** On a snow day, each hour adds its share of the day's snowfall (`AccumSWE / 24`), of which branches catch a fraction until they're full. So trees whiten through the storm instead of all at once.
- **Warmth melts it.** Degree-hours above freezing at the cell's temperature (the base temperature lapsed by elevation, as for the ground), faster in direct sun (the same exposure × horizon visibility as `meltHour`). Low, sunny slopes clear first; high, shaded, north-facing forest stays loaded.
- **Sun sublimates it, slowly, even below freezing.** A small loss in direct sun on clear days, so a cold bluebird week still slowly clears the south faces.
- **Wind shakes it off.** A wind day (the `weatherEventWind` roll in `applyDailyWeather`) sheds most of the load across the map.
- **Rain washes it off.** A rain hour clears it.

**Drawn from the load.** A per-cell R8 texture of load as a fraction of capacity, uploaded when it changes (at most hourly; 150k bytes on Boreal). `static.frag` samples it by world position for foliage instead of `uCanopySnow`, keeping today's per-tree noise and bough banding. `CanopySnowFor` and `uCanopySnow` go.

**Saved** with the terrain (breaking the save format is fine pre-release). A new or imported map starts from the snowpack's recent snowfall on its start date (the snow layer already computes `recent` per cell in `runSnowLayer`), so an opening day after a storm opens with loaded trees.

## Steps

1. **The load and its hourly update.** `Terrain` gets the per-cell canopy load; `tickHourly` runs the bough pass every hour (snowfall, melt, sublimation, rain), split across cores like the snowpack passes; the daily weather update applies the wind shed. Saved and loaded; seeded from the snowpack's recent snow. Check headless: through a storm the load rises hour by hour; afterwards it lasts days on cold shaded cells and a day or less on low sunny ones; a wind day and a rain day clear it.
2. **Draw it.** The canopy texture and the shader change; the editor shows the seeded load. Check with screenshots over a storm cycle on Boreal: mid-storm, the morning after (fully white), two days on (white up high and in the shade, green low and sunny), after a wind day.
3. **Tune by eye.** Capacity, catch fraction, melt and sublimation rates, and how much load reads as "white", against photos of real forests after storms.

## Ideas, not planned

- Clumps falling from the branches as it sheds (a few short-lived particles where load drops fast).
- Rime and "snow monsters" on exposed high trees in cold, windy, foggy weather.
- Guests caring: a powder hound loving a loaded forest, tree wells deeper under heavy boughs.

## Log

- 2026-10-08: Planned with the user; unranked.

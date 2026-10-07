---
title: Moguls
kind: plan
status: planned
---

# Moguls

Moguls that form where guests actually ski, look like a mogul field, soften under new snow, come off where the cat actually grooms, and matter to how guests ski and choose runs. Today they're a single number per 5 m cell that grows with time spent on it and is drawn as random shading. Part of [[Snow]] and [[Skiing]]; feeds the moguls taste in [[Snow Tastes]].

## Why

Read on 2026-10-07:

- **One number per cell, grown by time on it.** `wearSnowUnderfoot` (`internal/sim/skiing.go`) adds 0.005 × (1 − grooming) to `Cell.MogulSize` for every second a skier spends on a cell with over 30 cm of visible snow. That's about 200 skier-seconds, or 200–400 passes, to full moguls. Speed, turning, and slope don't count, so moguls form as fast on flats and runouts as on steeps.
- **Removed by grooming, melt-out, avalanches, and earthworks, but not by snow.** Each cat pass halves them (`groomMogulDecay`), across the whole cell. New snow buries grooming and tracks (`pushSnowLayer`) but leaves moguls untouched.
- **They slow skiers and nothing else.** Friction rises up to 1.6× base and 1.1× edge. They don't touch balance or falls, and steering neither seeks nor avoids them; only the moguls taste reacts (thoughts, run verdicts).
- **Drawn as shading, not moguls.** `MogulSize` rides the terrain vertex at 5 m, and `terrain.frag` tilts normals with value noise (about 3 m features, 0.8 m amplitude) that fades with distance. Unlike real moguls, it's random rather than regularly spaced, isn't aligned to the fall line, has no troughs where the lines run, and has cell-shaped edges. Grooming has its own 1 m map of where cats drove (`GroomMap`), and tracks are stamped along each skier's path at 0.25 m in the surface detail's R channel (`SurfaceDetail`, whose B and A channels are free).

## Decisions

From the user, 2026-10-07: plan it, ranked after [[Snow Tastes]], and build Snow Tastes with moguls in mind (the moguls feature is one of the taste features, read the same way wherever a guest judges snow, so a richer mogul source plugs in without changing the taste code).

## The model

**A mogul map.** Mogul height at sub-cell resolution, alongside tracks: a channel of `SurfaceDetail` (B, at 0.25 m) or a map of its own at 1 m like `GroomMap`. `Cell.MogulSize` stays as the cell's average, so physics, tastes, and the trail conditions in [[Snow Tastes]] read the same number they do now.

**Formed where people turn.** Growth is stamped along each skier's actual path, weighted by how hard they're turning and by slope (little on flats and runouts, most on steep, ungroomed fall lines), and by how deep and soft the snow is. Heavy traffic on one line digs troughs on it.

**Softened and cleared.** A snowfall softens moguls by the same burial factor it applies to tracks and grooming (2 cm of water covers them). Grooming clears them only in the lanes the cat drove (`GroomMap`), so a narrow cat pass leaves the edges bumpy. Warm days round them off a little; hard freezes leave them icy.

**Drawn as a mogul field.** A bump pattern with mogul spacing (about 3–5 m, tighter where better skiers turn more), aligned across the fall line, with troughs along the lines skiers take. Shading at a distance; on the close mesh, real height from the detail displacement.

**Skied as moguls.** Moguls add rhythm: a balance cost that grows with speed and shrinks with skill, so beginners struggle and bump skiers flow. Steering reads them through the moguls taste: bump lovers drift toward mogul fields, bump haters toward the smooth sides. Run choice already reads them through trail conditions ([[Snow Tastes]] step 4).

## Steps

1. **Mogul map and growth.** The map, growth stamped along paths and weighted by turning, slope, and snow, and `Cell.MogulSize` as its cell average. Check headless on a steep ungroomed run versus a flat one: moguls on the steep fall line, little on the flat.
2. **Snow, grooming, and weather.** Snowfall softens; grooming clears its lanes only; warm days round, freezes ice. Check: a storm night softens a mogul field, and a narrow cat pass leaves bumpy edges.
3. **Drawing.** Mogul-spaced, fall-line-aligned bumps with troughs; real height close up. Check by screenshot on a skied-out black.
4. **Skiing.** The balance cost by speed and skill, and steering toward or away by taste. Check: beginners fall more on a mogul field, and bump lovers end their runs on more of it than bump haters.
5. **Docs.** [[Snow]], [[Skiing]], [[Grooming]], [[Snow Spec]].

Each step builds with `go build` and `go vet` and is judged headless or by screenshot. No Go tests.

## Open questions

- Whether the map goes in `SurfaceDetail`'s B channel (0.25 m, shares the track machinery) or its own 1 m map (cheaper, like `GroomMap`).
- Whether moguls are saved (tracks aren't; moguls take days to build, so probably yes).
- Whether the player can choose to leave a run ungroomed for moguls (a trail setting), which [[Grooming]] would need anyway for powder days.

## Log

- 2026-10-07: Planned with the user after reviewing how moguls work today. Ranked after [[Snow Tastes]].

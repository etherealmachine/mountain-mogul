---
title: Moguls
kind: plan
status: done
---

# Moguls

Moguls that form where guests actually ski, look like a mogul field, soften under new snow, come off where the cat actually grooms, and matter to how guests ski and choose runs. Before this plan they were a single number per 5 m cell that grew with time spent on it and were drawn as random shading (Why, below); how they work now is in [[Snow Spec]] (Moguls). Part of [[Snow]] and [[Skiing]]; feeds the moguls taste in [[Snow Tastes]].

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

1. Done: **Mogul map and growth.** `world.MogulMap` (`mogul_map.go`) is its own map at 1 m per pixel, not a `SurfaceDetail` channel: moguls are 3–5 m apart, so 1 m is enough, and it's cheaper. Pixels are 16-bit, because a tick's growth rounds away at 8 bits. It's saved with the game at 8 bits (`moguls`), and saves without it fill evenly from each cell's `MogulSize`. Each cell's `MogulSize` is its pixels' average, so physics, tastes, and trail conditions read what they did. Anything else that sets `MogulSize` directly (grooming, melt-out, avalanches, earthworks) is reconciled the next time the map touches the cell, or before a save (`SyncMoguls`): the cell's pixels are scaled to match.

   `growMoguls` (`sim/skiing.go`) replaces the old growth of 0.005 a second for any skier on any cell. Growth is stamped about 1 m around the skier at 0.03 a second, multiplied by:
   - turning: how far their line crosses the fall line, full at 45°
   - slope: none below 5°, full from 20°
   - snow: at least 0.1 m of water, and 0.3 on crust, boilerplate, or frozen granular
   - 1 − grooming

   Each pixel is capped by slope, at 0.25 on the gentlest slope that grows moguls rising to 1 at 20°, so a green gets bumpy but never becomes a bump run. Changed with the user while checking: they expected Boreal's gentle green to get moguls after about a week ungroomed. The first version (10° to 25°, no cap, a 30 cm visible-depth gate) grew almost nothing and stalled after two days, because the snowpack's over-settling (Bugs, in [[Next Steps]]) left the run under 30 cm of visible depth while it held 120–165 mm of water. So the gate is on water, not depth.

   Checked on the user's Boreal save, a week ungroomed. Average mogul size by slope at day 7 (biggest cell): 5–8° 0.04 (0.27), 8–11° 0.11 (0.53), 11–14° 0.16 (0.63), over 14° 0.31 (0.77), growing steadily from day 1. Flats stay smooth. The biggest moguls off the run (0.56, at 14°) are just beside it, where taste steering takes some guests off the edge.
2. Done: **Snow, grooming, and weather.** Snowfall fills moguls in proportion to its water, all the way at 0.1 m (about a metre of new snow), not by the 2 cm burial factor that covers tracks: moguls are deeper than tracks, and at 2 cm any snowy day would wipe a week's moguls (`mogulFillSWE`, `pushSnowLayer`). A cat flattens the moguls under its tiller as it grooms, easing back to untouched over a metre past the tiller's edge (`FlattenMogulSwath`, called beside `ClearTrackSwath`); the old whole-cell halving (`groomMogulDecay`) is gone, so the cells beside a lane keep theirs. A warm clear day rounds them off by 10%, a day of rain by 20%; a hard freeze leaves them as they are, icy through the snow kind (`ScaleMoguls`).

   Checked on the user's Boreal save with every run and the two cells beside it set to 0.6: the debug storm (2.5 cm of water) took them to 0.45; that night's cats left the runs at 0.002 (99% of pixels flat, a few corners past the last lane untouched) and the cells beside the runs at 0.40.
3. Done: **Drawing.** The mogul map goes to the GPU at 1 m (`R16`) with each cell's fall line (`RG8`, smoothed over 3×3 cells, `RefreshMogulFallLines`), uploaded where it changed each frame (`FlushMoguls`, which also catches cells melt-out or avalanches cleared). `assets/shaders/mogul.glsl`, shared by the tessellation and fragment stages, draws mounds pushed up and troughs carved down as far, joined over saddles, so the field is one rolling surface rather than separate bumps (changed with the user: mounds on a flat floor read as isolated bumps). The mounds sit in a staggered lattice 6 m apart across the fall line and 7 m down it, so the lanes between them run diagonally. Each 12 m tile lays the lattice out along its own fall line and blends into its neighbours, two octaves of warp keep the rows from looking ruled, and each mound gets its own size. Height runs ±0.45 m × size, crest to trough bottom: the tessellation raises it up close, the fragment shader lights it from the same surface, and past about 2 m per pixel it gives way to a slight darkening from the trough shadows. `world.MogulHeightAt` mirrors the shader exactly (integer hashes, no float noise), and `VisualElevationAt` adds it, so skiers ride the drawn bumps. The old random value-noise bumps are gone. Not done: spacing that tightens where better skiers turn, and troughs dug along each skier's actual line; the lattice's lanes stand in for both.

   Checked by screenshot on Boreal's steepest run (23°) with full moguls: an irregular staggered field of mounds up close, a dotted mogul run from far off. A week's natural growth (about 0.3 on the steeper green) draws the same pattern at a third of the height.
4. Done: **Skiing.** Moguls drain balance by 0.6 a second × the size underfoot (read from the 1 m map) × speed ÷ 6 m/s × (1 − skill)², eased by up to half for bump lovers (`mogulStress`, `sim/skiing.go`); base recovery is 0.15 a second. So a beginner on big moguls at 5 m/s goes down in about seven seconds, an intermediate at 8 m/s about breaks even, and an expert barely feels them. Steering needed nothing new: the taste term already scores the moguls along each candidate line. Slowing down and steering clear came next (step 6).

   The first cost, 0.25 with skill taking off up to 85% linearly, added no beginner falls: beginners ski the gentle green at 4–5 m/s, where the moguls are small and the drain never beat recovery. Checked headless over two days on the user's Boreal save, cats off, moguls laid by slope (0.8 × (slope − 5°)/15°) in 15 m patches on the runs. Falls per guest, without moguls → with: beginners 0.28 → 0.58, intermediates 0 → 0.30, advanced 0 → 0. Mean mogul size underfoot on the runs: bump lovers 0.18, bump haters 0.14.
5. Done: **Docs.** [[Snow Spec]] has a Moguls section (map, growth, removal, skiing, drawing, save); [[Snow]], [[Skiing]], and [[Grooming]] say what moguls do.

6. Done: **Slowing down and steering clear** (added with the user after step 5). Guests aim for up to half their speed less on full moguls, less so with skill (an expert slows by 40% of that) and with a love of bumps (`mogulSpeedScale`). When steering, everyone's moguls taste is lowered by 0.6 × (1 − taste) (`mogulSteerAversion`), so everyone but a bump skier heads for the smoother side of a mogul run, while a guest who loves moguls outright keeps the full pull. An icy or treed side still loses: ice is disliked by every archetype, and steering keeps 10 m clear of tree stands. Steering reads moguls from the 1 m map at each sample point, so the less skied-out side of a run shows up.

   Checked headless over two days on the user's Boreal save, cats off, moguls by slope in 10 m stripes down the runs (alternate stripes at a fifth of the size). Time on big moguls (over 0.3), without → with: bump lovers 37% → 31%, neutral 26% → 15%, haters 21% → 18%; beginners' falls 1.03 → 0.78 each, and guests are slower on moguls than off them. A first test with only the outer cell of each run smooth showed no change: Boreal's runs are lined with trees, which steering keeps clear of, the "unless there are trees" case. A flat aversion of 0.4 or 0.6 also took bump lovers off the moguls, hence the fade.

Each step builds with `go build` and `go vet` and is judged headless or by screenshot. No Go tests.

## Open questions

- Whether the player can choose to leave a run ungroomed for moguls (a trail setting), which [[Grooming]] would need anyway for powder days.

## Log

- 2026-10-07: Planned with the user after reviewing how moguls work today. Ranked after [[Snow Tastes]].
- 2026-10-07: Step 1: the 1 m mogul map, growth along skiers' lines by turning, slope, and snow.
- 2026-10-07: Step 1 tuned with the user: growth from 5° to 20° with a size cap by slope, gated on snow water; a week ungroomed leaves Boreal's green bumpy.
- 2026-10-07: Step 2: snowfall fills moguls by its water (full at 0.1 m), cats flatten only under the tiller, thaws and rain round them off.
- 2026-10-07: Step 3: moguls drawn as a fall-line mogul field from the 1 m map, with real height that skiers ride.
- 2026-10-07: Step 4: moguls cost balance by size, speed, and skill; beginners fall twice as often on a mogul field, experts no more.
- 2026-10-07: Step 5: docs. Done.
- 2026-10-07: Step 6, added with the user: guests slow down in moguls and steer for the smoother side unless they love bumps.

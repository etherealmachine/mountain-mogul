---
title: Snow
kind: system
status: shipped
---

# Snow

Every terrain cell has a two-part snowpack: a consolidated season base and an active top layer. The top layer is one of a handful of kinds (powder, packed powder, cement, wind slab, crust, boilerplate, slush), and the kind sets friction, edge grip, and how it renders. Surface height is ground plus snow depth, so deep snow raises where guests ski and where lifts stand.

Each layer has its own density, and depth is its water over that. New snow lands at its kind's density (powder 0.12, machine snow 0.45) and settles each day toward 0.38 on top and 0.45 in the base, faster on a day that thaws, so a week-old powder layer is at about 0.27. Burying a layer in the base keeps its density, and a change of kind (wind, sun, skiers) doesn't change it. A cat packs only the top 35 cm to 0.45, so a metre of powder grooms to about 75 cm. Code: `SettleSnow`, `GroomCompact`, `FoldTopIntoBase` in `internal/world/terrain.go`.

The pack changes through:

- [[Weather]]: new snow, rain, freeze-thaw, wind, and hourly melt from sun and slope
- [[Snowmaking]]: machine snow on runs on cold nights
- [[Grooming]]: resets the top to packed powder with corduroy
- [[Skiing]]: traffic packs powder down, wears off grooming, and builds moguls
- [[Avalanche]]: strips a slope and dumps debris below

Moguls build up at 1 m resolution where guests turn on steep ungroomed snow, and new snow, thaws, and the snowcat's tiller take them away again. They're drawn as a mogul field guests ride over, and cost unskilled guests their balance ([[Moguls]]).

A lift whose base has no snow goes on hold (see [[Lifts]]).

Spec: [[Snow Spec]].

Snow that varies with sun and aspect from the start, and looks less plastic, is planned in [[Terrain Realism]].

## Log

- 2026-10-01: Layers, kinds, weather transitions, sun-aware melt, traffic, moguls, and the 1 m track texture are in.
- 2026-10-06: Snow sheds from steep ground and rock down the fall line after Auto snow, each day's snowfall, and Add Storm, and the renderer draws snow off rock ([[Ground Materials]]).
- 2026-10-06: Snow falling on an open lake disappears; lakes freeze and thaw daily from the weather ([[Creeks and Lakes]]).
- 2026-10-07: Moguls rebuilt: a 1 m map grown where guests turn, filled by snow, flattened by the cat, drawn as a mogul field, and costing balance ([[Moguls]]).
- 2026-10-08: Skier tracks fade in the shader by the minutes since they were skied, instead of a whole-texture fade every 30 s ([[Fast-Forward Performance]]).
- 2026-10-08: Snow changes mark their cells, and the renderer re-uploads only those 16-cell tiles instead of the whole map each time ([[Fast-Forward Performance]]).
- 2026-10-14: Layers keep their own density and settle day by day; the base is no longer 0.75 (nearly ice), and grooming, sun and skiers no longer crush a whole layer at once. Fixes snow compacting far too fast: on the Boreal Goals Test, a groomed run's metre of fresh powder now grooms to 0.75 m (was 0.27), and an ungroomed cell keeps 0.94 m of its 1.15 m across a rollover with new snow (was 0.89 to 0.41). One headless day: 2.88★ and 647 falls against 2.90★ and 578 before (one seed).

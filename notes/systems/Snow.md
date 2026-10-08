---
title: Snow
kind: system
status: shipped
---

# Snow

Every terrain cell has a two-part snowpack: a consolidated season base and an active top layer. The top layer is one of a handful of kinds (powder, packed powder, cement, wind slab, crust, boilerplate, slush), and the kind sets friction, edge grip, and how it renders. Surface height is ground plus snow depth, so deep snow raises where guests ski and where lifts stand.

The pack changes through:

- [[Weather]]: new snow, rain, freeze-thaw, wind, and hourly melt from sun and slope
- [[Snowmaking]]: adds base under the guns on cold nights
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

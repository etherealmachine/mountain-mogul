---
title: Snowmaking
kind: system
status: shipped
---

# Snowmaking

Snowmaking is bought for a run: pipe and hydrants along it, $250 a metre, from the run's popup ([[Snowmaking on Runs]]). Once it's in, the run has a switch and a target depth (60 cm by default, 20 cm to 3 m). While the lifts are stopped (and up to two hours before they open, so the cats can groom over it), every cell of a switched-on run that's under its target gets machine snow at a rate set by the air there: the base's temperature lapsed to the cell's height ([[Weather]]), nothing at −2 °C, full rate at −10 °C or colder. Full rate is 8 mm of water an hour, so a cold twelve-hour night adds about 19 cm. It costs $2 per cubic metre of water made into snow (about $4,800 for a cold night on a kilometre run 25 m wide), charged with the day's costs as "Snowmaking"; a switched-off run, or one at its target, costs nothing.

Machine snow is its own kind ([[Snow]]): 0.50 density, firm and fast, grippy. It takes in the surface layer it's laid on (so a night's snowmaking never leaves a run shallower), clears the corduroy, packs down to packed powder under skiers or a cat, stays put through cold, clear and windy days, and turns to corn on a warm one. The season story it carries is in [[Vision]]: opening before natural snow and staying open after it melts.

Not in the demo. The placed snow guns are gone; saves drop theirs.

Code: `internal/world/snowmaking.go`, `internal/sim/snowmaking.go`, the popup rows in `internal/scene/trail_snowmaking.go`.

## Log

- 2026-10-01: Guns, the temperature gate, and base buildup are in.
- 2026-10-14: Replaced by snowmaking on runs ([[Snowmaking on Runs]] step 1): bought by the metre, made by temperature and altitude to a target depth while the lifts are stopped, paid by the water; machine snow as its own kind; the placed guns removed.

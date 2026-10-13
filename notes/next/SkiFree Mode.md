---
title: SkiFree Mode
kind: plan
status: planned
---

# SkiFree Mode

Take a skier and ski the resort yourself. In SimGolf, playing your own course was fun by itself and was the best way to feel whether a hole worked; this does the same for runs. The name is a nod to SkiFree, yeti included. Part of [[Skiing]]; the yeti ties in with the Yeti sighting under Easter eggs in [[Next Steps]].

## Why

The user, 2026-10-10: playing the golf was a big part of SimGolf's fun, so the resort should have a mode where you ski it. Everything the guests ski on is already simulated (pitch, snow underfoot, moguls, trees, falls), so the run should feel like the mountain the player built.

## Sketch

- **Drop in.** Pick a lift top (or ride the lift up) and take control of one skier. The game keeps running around you: guests, lifts, cats, weather.
- **Same physics as the guests.** Player input replaces the steering in `internal/sim/skiing.go`: turn left and right, tuck, skid to check speed, stop. Edge grip, balance, moguls, trees, and falls (yard sales included) work exactly as they do for guests, at an expert's skill or one you pick.
- **The resort reacts.** Guests dodge you the way they dodge each other; crash into one and they think about it. 
- **The yeti.** Leave the boundary, or ski too far into the backcountry, and the yeti comes for you, like SkiFree.
- **Feedback for building.** Optionally show what a guest would think of the run you just took (too steep for a green here, icy there), so playing it is also a way to check it.

## Decisions

From the user, 2026-10-13:

- **A button in a running game.** "Ski it" drops you into the live resort; Esc hands control back and you're managing again. No start-menu mode.
- **The clock runs at 1×** while you ski: guests, lifts, cats and weather carry on around you in real time (a game hour is 15 real minutes), and the chosen pace comes back on Esc.
- **Chase camera**, behind and above the skier looking downhill. The first-person view (V) stays as it is.
- **Goals:** the yeti past the boundary, timed runs, and riding the lift back up with the guests. The run verdict is not in.
- **No boundary, no yeti.** A map with no ski-area boundary drawn never sends the yeti.
- **You just get up.** A fall is a fall like a guest's, but no patrol and no injury: you get up and carry on.
- Not in the [[Demo]]: the mode's code sits behind `!demo` like the other left-out features.

## Steps

In order. Each step builds with `go build` and `go vet` and is judged in the running game.

1. **Take control.** A "Ski it" button in the lift popup puts a player skier (an advanced guest, skill 0.8) at the top station's apron and follows them. Player input builds the `Decision` in place of `decide()` (the special case goes in `decideRange`, `sim/ski_parallel.go`; `perceive` still runs): left and right (A/D or arrows) turn the `DesiredHeading` at the carve rate, held with Shift for the pivot rate from `turnRates`; down (S) brakes with `Scrub`; space stops, and the player may come to a full stop (today's 1 m/s creep floor is for guests). `applySkier` runs unchanged, so grip, skids, moguls, trees, balance and falls are the guests' own, except that the player is never injured and patrol never comes: after a fall they get up and carry on. The player guest skips planning (`tickPlanning`) and `tickLocomote`'s target, never goes home, and isn't counted in arrivals, reviews or the rating. Time is fixed at 1× while skiing (`SetTimeScale(1)`); Esc removes the skier and restores the pace.
2. **Chase camera.** A perspective camera behind and above the skier, beside `applyFirstPersonCamera` (`scene/scenario.go`): looks along the heading and downhill, eased so turns don't whip it round, lifted to stay above the terrain. Scroll sets the distance.
3. **Ride the lift up.** Stopping in a lift's line corral joins the line (`lift.JoinLine`, `Queued`); the chairs do the rest (`loadChair`, `tickRiding`, `startUnloading`). Guard `advancePlan` and `aimUnload` for the player, who has no plan; the unload peel-off and its fall chance stay. No ticket check: it's your resort.
4. **Timed runs.** Leaving a marked run's top starts a clock and reaching its bottom stops it; a small HUD shows the time and the run's best. Bests are kept per run in the save. Then a ghost: record the best run's frames and replay them as a see-through skier on later attempts.
5. **The yeti.** A few seconds past the ski-area boundary (`SkiAreaMask`, `world/ski_area.go`) the yeti appears uphill and chases you, faster than you; it gives up if you get back inside quickly enough. If it catches you: a yard sale, a short scene, and you're put back at the last lift top. With no boundary drawn there's no yeti. Needs a yeti model ([[Model Pipeline]]). The Yeti sighting under Easter eggs can reuse it.

Later, not planned: a gamepad (GLFW has no gamepad polling in `engine/input.go` yet), picking your skill, the top-down SkiFree view, and race gates.

## Open questions

- Do guests think about being crashed into by the owner?

## Log

- 2026-10-10: Idea from the user, unranked.
- 2026-10-13: Picked as the next feature and planned with the user: in-game button, 1× clock, chase camera, the yeti, timed runs, riding lifts.
- 2026-10-13: No boundary, no yeti; the player gets up after a fall, no patrol (the user).

---
title: Crowd Scale
kind: plan
status: partial
---

# Crowd Scale

What it would take for the sim to carry a big resort's crowds (10,000 visitors a day) and still fast-forward a whole day in about a minute. Follows [[Fast-Forward Performance]]; part of [[Skiing]] and [[GOAP]].

## Measured (2026-10-08)

A throwaway driver loaded the user's latest save (Boreal, three lifts, ten runs), put N guests on the mountain through the normal spawn path at 9:30, ran the sim headless at 360× (what a 24-hour day in a minute needs: 21,600 sim seconds, or about 11,000 of the sim's 1/30 s steps per real second) and timed it. Rendering not included.

| Guests on the mountain | Before this pass | After | A 24 h day takes (after) |
|---|---|---|---|
| ~35 (the save's own) | 354× | 470× | 46 s |
| 250 (mostly skiing) | 57× | 56× | 6.5 min |
| 1,000 (most waiting: lift lines are full) | 23× | 33× | 11 min |
| 2,500 | 10× | not re-run | ~37 min before |

- **A skiing guest costs about 3.3 µs per step**: steering (`decide`, 42% at 250 guests), re-planning the route round the trees (`planSkiRoute`'s A*, 27%), and goal planning (15%), each run for every skier every 1/30 s. A guest riding, queuing, or walking costs about 1 µs.
- **The pass's fixes**: building tile counts are cached (`TileCount` was a quarter of the time with 1,000 guests: every plan's world snapshot walked every building's tiles through maps), and the on-a-building check rejects on the ground's bounds first. Parallel steering now starts goroutines only for 64 skiers or more per core: below that, waking threads every 1/30 s step cost more than the steering it split (on macOS most of the CPU went to waking threads).

## What 10,000 a day means

A busy day of 10,000 visitors staying five or six hours puts 6,000 to 7,000 guests on the mountain at the peak, about half of them skiing at any moment. At today's costs that's about 15 ms per step against a 33 ms step: **the sim could just about run at twice real time**, a ski day taking about 45 minutes. A day in a minute is about 150× faster than that.

It also needs the mountain for it: ten or so high-capacity lifts and the terrain to match (`TerrainCapacity`). With three lifts the save's lines fill at a few hundred guests, which is why most of the 1,000 were waiting. (Its first day drew about 100 visitors; that's [[Demand]] and capacity, not speed.)

## Options, roughly in order of payoff

1. *Rejected by the user (2026-10-08): two code paths, and the path matters for snow and collisions.* **Simulate off-screen guests coarsely.** At fast-forward, or out of view, a guest doesn't need 30 steering decisions a second: ski a run as one event (its time from length, pitch, skill, and conditions; falls, thoughts, and verdicts sampled from the same rules), and move them along the run's centre line ([[Trail Network]] gives every run one) so they're still drawn in a plausible place. Full simulation only for guests near the camera at low speed, or the followed one. The only option that reaches a day in a minute at 10,000 visitors; changes how guests are simulated, so it needs care that coarse and fine agree (run times, fall rates, verdicts).
2. **Steer less often.** Rescore the fan every few steps instead of every step, and keep a route round the trees until the guest leaves it instead of re-planning (`planSkiRoute`). Two to four times on skiers; checked with the testbeds and tree-hit counts, as for the hazard cache.
3. **Bigger steps at speed.** 1/10 s instead of 1/30 s for skiers at fast-forward: about three times, if tree hits, falls, and lift loading hold up.
4. Done in part (2026-10-08): **Real parallelism.** A persistent worker pool over all guests, not per step goroutines: worthwhile once there are thousands of skiers (each step's work then far outweighs waking threads); up to 5 to 8 times on 10 cores.
5. **Smaller wins**: serving lift lines without a sort every step, the per-step tick of guests who are just waiting, goal planning's world snapshot.

2 to 5 together might give 20 to 50×: enough for thousands of guests at fast-forward speeds, not for a day in a minute at 10,000. Only 1 gets there.

## Simplifying the one sim (2026-10-08)

The user, 2026-10-08: no coarse sim for off-screen guests. A skier's path drives snow quality and skier-on-skier collisions, and two code paths are complexity they don't want. Explore simplifying the one skiing sim instead, dropping fidelity where it's good enough: coarse everywhere.

Measured with temporary knobs on the user's latest save: 250 guests, 9:30 to 15:30, seed 1 against a baseline; a second baseline seed gives the noise. "Heat" is the cosine similarity of where guests skied (time spent per cell, sampled each sim second) to the baseline's, the closest stand-in here for snow wear.

| Variant | Speed | Rides | Tree hits | Falls | Close passes (< 1.5 m) | Heat |
|---|---|---|---|---|---|---|
| Baseline, seed 1 | 50× | 1,575 | 15 | 8,711 | 106,698 | — |
| Baseline, seed 2 | 50× | 1,528 | 20 | 8,540 | 106,708 | 0.990 |
| Step 1/15 s | 63× | 1,549 | 17 | 8,984 | 94,899 | 0.988 |
| Step 1/10 s | 72× | 1,534 | 13 | 8,424 | 97,257 | 0.987 |
| Steering rescored every 3rd step | 52× | 1,578 | 21 | 9,095 | 99,688 | 0.990 |
| Every 6th step | 55× | 1,556 | 11 | 8,614 | 89,842 | 0.985 |
| Fan of 5 × 5, centre line only | 54× | 1,564 | 33 | 9,291 | 98,669 | 0.880 |
| 1/15 s + every 3rd + 5 × 5 | 70× | 1,606 | 26 | 9,258 | 108,822 | 0.884 |
| 1/10 s + every 3rd + 5 × 5 | 78× | 1,577 | 32 | 8,998 | 116,307 | 0.881 |

- **A bigger step is nearly free**: at 1/10 s everything measured stays within the seed-to-seed noise, for 44% more speed. It's the one simplification worth taking. The catch: at 1× a guest would move ten times a second, so the renderer has to draw them between their last two positions or they'll visibly hop; or the step grows only at fast-forward (1/30 s up to 4×, 1/10 s above), which is still one code path, only a different dt.
- **A smaller steering fan changes where people ski** (heat 0.88, far outside the noise) and doubles tree hits: keep the fan.
- **Rescoring steering less often** gains little at this crowd (about 100 skiing at once); worth revisiting with more skiers.
- **Bigger steps stop paying at about 1.5×** because the remaining cost isn't per step. At 1/10 s, two thirds of the main thread is the walking pathfinder (`Pathfinder.FindPath`, a map-based A*, run at every plan step start and replan), then route planning round trees (on the worker pool). So the next levers are replanning less and a cheaper `FindPath` (arrays instead of maps), not the skiing itself.
- **Two things that look wrong** (in Bugs, [[Next Steps]]): about 8,700 falls a day for 250 guests, some 35 each; and so much replanning that pathfinding dominates. They may be one problem (a fall or a full lift line sending guests back to the planner). Worth diagnosing before tuning speed further, since both are also fidelity.

## Log

- 2026-10-08: Measured with the user's latest save and written up; tile counts cached, building checks bounded, parallel steering's threshold raised.
- 2026-10-08: Worker pool built (`sim/worker_pool.go`): one goroutine per core, started once, spinning briefly between batches and then sleeping; the caller takes chunks too, so a batch never waits on a sleeping worker; each batch has its own counters. Steering runs on it in chunks of 16 skiers, and so do route plans round the trees, now planned for every guest about to move in a pass before the serial one (`planRoutes`; always there, parallel or not, so a seeded run doesn't depend on core count; each route sees the terrain as the step began). Serial and parallel builds end identical; the race detector is quiet. On the user's latest save at 360×: 250 guests (100 skiing) 55× → 69×; 1,000 (lift-bound, about 100 skiing) 33× → 37×; 2,500 18× (10× before this and the previous pass). Spinning longer, to bridge the half millisecond between steps, measured slower (busy workers crowd the main goroutine); so did fewer workers. Most of a step is still serial (planning, lines, the tick of guests who are waiting), so the pool can't give much more until more of the step moves into parallel phases, and option 1 remains the only way to a day in a minute at 10,000 visitors.
- 2026-10-08: Explored simplifying the one sim (the user rejected coarse off-screen sims): a 1/10 s step is nearly free in fidelity (+44%); a smaller fan isn't; rescoring less often gains little. Falls and replanning look wrong.
- 2026-10-08: Diagnosed the two oddities (Bugs, [[Next Steps]]): the falls are beginners sent down the fall line by free-ski plan steps, falling every ~6 s; the pathfinding cost is failed searches to a door enclosed by its own building, each flooding the map, not frequent replanning.
- 2026-10-09: First pass toward 1,000 a day ([[Season Calendar]]; the user's goal is 10,000 a day in 80 s, 1,000 first, accepting some change in falls, the odd skier clipping a tree, and coarser tracks). Measured on the Boreal Goals Test at the Christmas peak (about 850 on the mountain, 11 am), headless: 43 s per game hour → 8.7 s; a whole Christmas day (998 visitors) about 400 s → 83 s, an ordinary day (about 430) 36–42 s. Ordinary days play the same: 3.26★ and 3.21★ against 3.23★ and 3.22★, falls 217 and 342 against 251 and 405, rides 3.3 a guest against 3.2. What did it:
  - **A 0.2 s step** (was 1/30 s): about 3× on everything that runs per step.
  - **Route searches**: buffers reused with generation stamps instead of allocated and cleared (tens of KB, ten thousand times a game hour); the open list as plain values; a weighted estimate (2.5×), half the cells expanded; re-planned every 10 s or 50 m of target movement (was 5 s, 25 m); a guest on a run near its centre line follows the line instead of searching.
  - **The planner's closed set** keyed by a struct, not a formatted string.
  - **Boarders' post-ride plans** made in batches of 16 across cores (a rider needs one only at the top).
  - **Trail centre lines** indexed in 25 m buckets (nearest-sample lookups, twice a step a skier); building door steps cached; the planning snapshot kept off the heap.
  - **Tracks** drawn as 2×2 splats every second pixel (were 3×3 on every pixel); this turned out to be only about 2%.
  The CPU profiler on macOS misattributed time badly (thread wake-ups, and track drawing at 14% that was really 2%); wall-clock timers per phase of the step were what showed the cost. At the peak the step is now roughly route searching 2, movement 2, steering 1.5 (parallel), planning 1, applying moves 0.8, lifts 0.4, cats and patrol 0.4 seconds per game hour. Rendering isn't included.
- 2026-10-09: More of the step on the cores. A skier is queued for the parallel pass straight from the serial one, with their goal and seed; the parallel pass works out their way round the trees (route searches now spread across the same chunks), whether they walk, arrive, or ski, their perception, and their steering, writing only the skier's own route; the walk or the run is applied after, in order. Plan checks run across cores before the serial pass (from the world as the step began): the serial pass runs `tickPlanning` only for guests whose step finished or can't go on. The route pre-pass is left to guests with skis off. The race detector is quiet. Christmas peak 8.7 → 6.6 s per game hour; a whole Christmas (995 visitors, 3.04★) 83 → 63 s; ordinary days 29–35 s at 3.23★ and 3.23★. Still serial: applying each skier's move (snow wear, tracks, runs, falls), mood, lifts, cats and patrol; rendering isn't measured yet.
- 2026-10-09: Measured rendered at the fastest speed (80 s a day): a 1,000-visitor Christmas in 102 s (sim 63, CPU drawing 12, GPU 23, with the harness waiting on the GPU each frame); the user: close enough. Ten times the guests isn't reachable by speed-ups alone (perhaps 3–4× more); [[Groups]] planned as the way there, keeping tracks, collisions and terrain-grounded skiing per body.

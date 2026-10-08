---
title: Demo
kind: plan
status: planned
---

# Demo

A playable demo of a few hours: the [[Boreal]] tutorial, then [[Kirkwood]]. It's the free Steam demo, a separate build with the extras stripped out ([[Release]]). Close to ready; what's left is wrapping up features and polishing the graphics and UI. Ranked first in [[Next Steps]].

## Decisions

From the user, 2026-10-07:

- **Two scenarios.** [[Boreal]] as the tutorial (about an hour: a lift, then three; a lodge with tickets, food, and rentals; opening; reading the rating), then [[Kirkwood]] (a couple of hours or more: beginners and experts on a big real-terrain map, parcels, grooming against moguls and powder, a season of weather).
- **Nothing is locked.** Every lift type and building the game has is available in both; expensive things are gated by their price. Land use (zoning) can later rule out things like hotels on a scenario's land, but the demo doesn't need it.
- **No snowmaking** in the demo.
- **One new lift type: the magic carpet**, to show lift variety in the beginner areas. No T-bars or rope tows yet.
- **No new services.** The demo has the services already built: [[Tickets]], [[Lounge]], [[Food Court]], [[Bar]], [[Rental Shop]], [[Ski Patrol]], and grooming ([[Grooming]]). Everything else in [[Services]] comes after.
- **No tutorial guidance.** A demo pop-up on start, then guests' thoughts tell the player what to fix.

## Steps

In order. Each step builds with `go build` and `go vet`, and gameplay is checked headless or by screenshot.

1. **Magic carpet.** A surface lift type for beginner areas: a short conveyor guests stand on, cheap, slow, low capacity, no chairs.
2. **Terrain expectations** (from the user's first playtest of a three-lift Boreal, 2026-10-07: ratings were poor, mostly guests unhappy that the terrain was too easy, though Boreal really is almost all easy intermediate). Guests should come expecting the terrain the resort has: readouts of skiable acres (needs a ski-area boundary; the import already has OpenStreetMap's), groomed acres, and marked runs by difficulty; demand weighing a resort's terrain mix, so an easy hill draws few experts; and a per-guest challenge preference (beginners and intermediates looking for a step up, experts happy cruising), so the experts who do come to Boreal are content there. Decided with the user: only marked runs count (the player controls them), the steep taste is the challenge preference (it also lets some experts at Boreal enjoy the powder beginners leave alone), and the ski-area boundary should be something the player draws over the OpenStreetMap layer like other features.
   - Done: a guest's run range (`runRange`, `sim/demand.go`): their skill level and the level they want, which is one easier with a steep taste of −0.3 or less (happy cruising) and one harder at 0.5 or more (after a challenge). A run's verdict uses the range: too easy below it, too hard above it, great anywhere in it.
   - Done: demand by terrain mix (`terrainMatch`): each marked-run cell counts fully in the guest's range, 0.3 one level easier, 0.1 two easier, not at all harder; half the marked terrain suiting a guest draws them fully.
   - Done: groomed terrain in the Resort overview, beside the marked terrain by difficulty that was already there.
   - Done: the ski-area boundary, drawn in the scenario editor over the OpenStreetMap layer (or copied from OpenStreetMap's where the import has one; Boreal's import has none), a rope line in the game (orange poles every 8 m along the outline with a drooping yellow rope, as resorts mark their boundary; gaps at buildings, lots, and roads; riding the snow; dimming at night), and its area in the Resort overview as "Ski area", beside marked, groomed, and by-difficulty runs ([[Scenario Editor]]).

   Checked headless over two days on the user's playtest save (marked runs 41% green, 49% blue, 8% black), against the old code: 132 arrivals against 160, advanced guests 8% against 13%; too easy 0.51 a guest against 0.97; great runs 1.86 a guest against 1.0. On their next save, with an ungroomed black run (18% black), advanced guests rose to 16%. The rating stays poor (15–32%): that's the balance step.
3. **Balance so the goals are reachable.** Started 2026-10-07 on the user's built Boreal test: a breakdown of each guest's score by cause found guests hitting the same tree over and over (−1.6 a guest, swamping everything; fixed, see [[Skiing]]). What's left per guest over two days: −0.115 trees (guests on long lines to a far lift steered through forest; since fixed with quick manoeuvres and forest routing, tree hits down from about 55 to 1–3 a day, see [[Skiing]]), −0.08 too easy, −0.07 cold with no lounge, −0.04 falls; +0.11 great runs, +0.02 drinks. Ratings 26% and 45%. Then: Calibrate the satisfaction ledger (tabled until now, [[Satisfaction]]) and [[First Week Balance]]: a headless three-lift Boreal sits around 30–44%, against a 70% goal. Includes rental shops (since [[Service Improvements]], a resort without one gets about a quarter fewer beginners) and the rolled needs' shares.
4. **Goals.** Boreal's targets from a headless run of a reasonable three-lift Boreal ([[Scenario Goals and Rules]] step 7); Kirkwood's goals, which need the rating broken down by skill; Kirkwood unlocking when Boreal is won (step 6).
5. **Snow bugs** visible in the first days: fresh corduroy turning to crust overnight, and snow compacting far too fast ([[Next Steps]] Bugs).
6. **Kirkwood map pass.** Hand-drawn parcels following the terrain, and checking the forest and the chair line ([[Kirkwood]]).
7. **Demo pop-up.** On starting the game: what this is, what's in it, and where to send feedback.
8. **UI polish.** Clear goal and rating feedback; named complaints, so the player knows what to fix ([[Next Steps]]); a build menu that reads well; a clear sign when the resort or its lifts are closed and the player has to open them (the user's playtest started with everything closed and nothing said so; no tutorial needed).
9. **Graphics polish.** [[Terrain Realism]] leftovers, [[Hiding the Grid]], and Kirkwood's frame rate (about 35 fps against Boreal's 42); models, and procedural roads and buildings (from the playtest; the skiers' tracks already look good).
10. **Fast-forward performance.** With only 50 guests, the top speeds were jumpy in the playtest; players will want to skip days quickly to earn money, so the sim needs to keep up at high speed. Done in [[Fast-Forward Performance]] (2026-10-08).
11. **Traffic you can see.** Traffic works (on the playtest save every car drove in from a map-edge entry, the first at 6:18), but about 15 cars an hour each spend some 5 clock minutes on the road, a second or two at high speed, so it read as cars appearing in the lot. Traffic is a big part of the game: busier, slower, or more visible arrivals.
12. **Menus and settings.** Start screen, save, load, and quit, and a settings check.
13. **The demo build.** A separate build (likely a `demo` build tag) without the editor, the other scenarios, debug flags, testbeds, or snowmaking; maybe obfuscated; packaged for Steam ([[Release]]).

## Open questions

- Whether Kirkwood is open from the start in the demo or unlocks after Boreal.
- What the demo pop-up says, and where feedback goes.

## Log

- 2026-10-07: Planned with the user: Boreal and Kirkwood, nothing locked (price gates), no snowmaking, the magic carpet as the one new lift, no new services, no tutorial guidance beyond a pop-up.
- 2026-10-07: The demo is the free Steam demo, its own stripped build (step 10); snowmaking is stripped from it ([[Release]]).
- 2026-10-07: The user's first playtest of a three-lift Boreal: terrain expectations added as step 2 (guests unhappy with easy terrain on a hill that's honestly easy); a closed-resort sign, fast-forward performance, visible traffic, and model and road polish added to later steps.
- 2026-10-07: Step 2 mostly done: run range from the steep taste, demand by terrain mix, groomed terrain readout; the ski-area boundary is left.
- 2026-10-07: Step 2 done: the ski-area boundary and its readout.
- 2026-10-07: Balance started: the repeated tree-hit bug found and fixed; the score breakdown recorded.
- 2026-10-07: Skiers turn and swerve properly now (skill-based carve and pivot rates, linked turns on every slope, swerves round trunks, towers, and skiers) and route round forest: tree hits on the Boreal test about 55 → 1–3 a day.
- 2026-10-08: Fast-forward performance profiled and pulled forward as [[Fast-Forward Performance]].

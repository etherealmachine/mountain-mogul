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
2. **Balance so the goals are reachable.** Calibrate the satisfaction ledger (tabled until now, [[Satisfaction]]) and [[First Week Balance]]: a headless three-lift Boreal sits around 30–44%, against a 70% goal. Includes rental shops (since [[Service Improvements]], a resort without one gets about a quarter fewer beginners) and the rolled needs' shares.
3. **Goals.** Boreal's targets from a headless run of a reasonable three-lift Boreal ([[Scenario Goals and Rules]] step 7); Kirkwood's goals, which need the rating broken down by skill; Kirkwood unlocking when Boreal is won (step 6).
4. **Snow bugs** visible in the first days: fresh corduroy turning to crust overnight, and snow compacting far too fast ([[Next Steps]] Bugs).
5. **Kirkwood map pass.** Hand-drawn parcels following the terrain, and checking the forest and the chair line ([[Kirkwood]]).
6. **Demo pop-up.** On starting the game: what this is, what's in it, and where to send feedback.
7. **UI polish.** Clear goal and rating feedback; named complaints, so the player knows what to fix ([[Next Steps]]); a build menu that reads well.
8. **Graphics polish.** [[Terrain Realism]] leftovers, [[Hiding the Grid]], and Kirkwood's frame rate (about 35 fps against Boreal's 42).
9. **Menus and settings.** Start screen, save, load, and quit, and a settings check.
10. **The demo build.** A separate build (likely a `demo` build tag) without the editor, the other scenarios, debug flags, testbeds, or snowmaking; maybe obfuscated; packaged for Steam ([[Release]]).

## Open questions

- Whether Kirkwood is open from the start in the demo or unlocks after Boreal.
- What the demo pop-up says, and where feedback goes.

## Log

- 2026-10-07: Planned with the user: Boreal and Kirkwood, nothing locked (price gates), no snowmaking, the magic carpet as the one new lift, no new services, no tutorial guidance beyond a pop-up.
- 2026-10-07: The demo is the free Steam demo, its own stripped build (step 10); snowmaking is stripped from it ([[Release]]).

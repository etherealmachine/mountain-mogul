---
title: Demo
kind: plan
status: planned
---

# Demo

A playable demo: the [[Boreal]] tutorial only (the user, 2026-10-12; Kirkwood was dropped). It's the free Steam demo, a separate build with the extras stripped out ([[Release]]). Close to ready; what's left is wrapping up features and polishing the graphics and UI. Ranked first in [[Next Steps]].

## Decisions

From the user, 2026-10-07:

- **Two scenarios.** [[Boreal]] as the tutorial (about an hour: a lift, then three; a lodge with tickets, food, and rentals; opening; reading the rating), then [[Kirkwood]] (a couple of hours or more: beginners and experts on a big real-terrain map, parcels, grooming against moguls and powder, a season of weather).
- ~~**Nothing is locked.**~~ Replaced 2026-10-12 (the user): the demo limits what the player can build. Left out of the demo: the snow gun, the gondola, heli and 6-pack lifts, the bar, the tent, the rental shop, and buying land. In: double, triple, quad and high-speed quad chairs; lodge and shed buildings; tickets, food court, lounge, patrol and garage; every transport tool; glade, trail and remove.
- **Boreal only** (the user, 2026-10-12). Kirkwood, the Goals Test and every other scenario stay out.
- **Stripped from the binary, sim code included** (the user, 2026-10-12): the left-out features' sim, render and save code is compiled out, not just hidden behind the menu. That also covers the editor, testbeds, the debug console, and the dev flags.
- **No snowmaking** in the demo.
- **No new lift types** (the user, 2026-10-10: the magic carpet is out of the demo for now).
- **No new services.** The demo has the services already built: [[Tickets]], [[Lounge]], [[Food Court]], [[Bar]], [[Rental Shop]], [[Ski Patrol]], and grooming ([[Grooming]]). Everything else in [[Services]] comes after.
- **No tutorial guidance.** A demo pop-up on start, then guests' thoughts tell the player what to fix.

## Steps

In order. Each step builds with `go build` and `go vet`, and gameplay is checked headless or by screenshot.

1. Dropped 2026-10-10 (the user): ~~**Magic carpet.**~~ Out of the demo for now; it stays under Surface lifts in [[Next Steps]].
2. **Terrain expectations** (from the user's first playtest of a three-lift Boreal, 2026-10-07: ratings were poor, mostly guests unhappy that the terrain was too easy, though Boreal really is almost all easy intermediate). Guests should come expecting the terrain the resort has: readouts of skiable acres (needs a ski-area boundary; the import already has OpenStreetMap's), groomed acres, and marked runs by difficulty; demand weighing a resort's terrain mix, so an easy hill draws few experts; and a per-guest challenge preference (beginners and intermediates looking for a step up, experts happy cruising), so the experts who do come to Boreal are content there. Decided with the user: only marked runs count (the player controls them), the steep taste is the challenge preference (it also lets some experts at Boreal enjoy the powder beginners leave alone), and the ski-area boundary should be something the player draws over the OpenStreetMap layer like other features.
   - Done: a guest's run range (`runRange`, `sim/demand.go`): their skill level and the level they want, which is one easier with a steep taste of −0.3 or less (happy cruising) and one harder at 0.5 or more (after a challenge). A run's verdict uses the range: too easy below it, too hard above it, great anywhere in it.
   - Done: demand by terrain mix (`terrainMatch`): each marked-run cell counts fully in the guest's range, 0.3 one level easier, 0.1 two easier, not at all harder; half the marked terrain suiting a guest draws them fully. Replaced 2026-10-10 by `terrainDraw`, the amount and number of suitable runs the running lifts serve ([[Boreal Tutorial]]).
   - Done: groomed terrain in the Resort overview, beside the marked terrain by difficulty that was already there.
   - Done: the ski-area boundary, drawn in the scenario editor over the OpenStreetMap layer (or copied from OpenStreetMap's where the import has one; Boreal's import has none), a rope line in the game (orange poles every 8 m along the outline with a drooping yellow rope, as resorts mark their boundary; gaps at buildings, lots, and roads; riding the snow; dimming at night), and its area in the Resort overview as "Ski area", beside marked, groomed, and by-difficulty runs ([[Scenario Editor]]).

   Checked headless over two days on the user's playtest save (marked runs 41% green, 49% blue, 8% black), against the old code: 132 arrivals against 160, advanced guests 8% against 13%; too easy 0.51 a guest against 0.97; great runs 1.86 a guest against 1.0. On their next save, with an ungroomed black run (18% black), advanced guests rose to 16%. The rating stays poor (15–32%): that's the balance step.
3. **Service staffing and quality.** Staff as a pool per room: headcount sets how fast a service serves, the wage sets staff morale, and morale sets quality, so a cheap, well-staffed food court and a short-staffed bad restaurant are both things to build ([[Service Quality]] steps 2 and 3; decided with the user 2026-10-08: formats, appetite, shopping, and variety wait until after the demo).
4. **Balance so the goals are reachable.** Started 2026-10-07 on the user's built Boreal test: a breakdown of each guest's score by cause found guests hitting the same tree over and over (−1.6 a guest, swamping everything; fixed, see [[Skiing]]). What's left per guest over two days: −0.115 trees (guests on long lines to a far lift steered through forest; since fixed with quick manoeuvres and forest routing, tree hits down from about 55 to 1–3 a day, see [[Skiing]]), −0.08 too easy, −0.07 cold with no lounge, −0.04 falls; +0.11 great runs, +0.02 drinks. Ratings 26% and 45%. Then: Calibrate the satisfaction ledger (tabled until now, [[Moments]]) and [[First Week Balance]]: a headless three-lift Boreal sits around 30–44%, against a 70% goal. Includes rental shops (since [[Service Improvements]], a resort without one gets about a quarter fewer beginners) and the rolled needs' shares.
5. **Goals.** Boreal's targets from a headless run of a reasonable three-lift Boreal ([[Scenario Goals and Rules]] step 7); Kirkwood's goals, which need the rating broken down by skill; Kirkwood unlocking when Boreal is won (step 6).
6. **Snow bugs** visible in the first days: fresh corduroy turning to crust overnight, and snow compacting far too fast ([[Next Steps]] Bugs).
7. **Kirkwood map pass.** Hand-drawn parcels following the terrain, and checking the forest and the chair line ([[Kirkwood]]).
8. **Demo pop-up.** On starting the game: what this is, what's in it, and where to send feedback.
9. **UI polish.** Done 2026-10-13 from QA notes: the Parkitect-style pass (translucent rounded panels, a picture dock from model thumbnails, round close buttons, tooltips, a three-pod top bar), the Resort Overview's Guest thoughts and Mountain capacity tabs removed, the game's Ground and Trails views removed, and the event feed cut to what the player would otherwise miss ([[UI and HUD]], [[Event Feed]]). Left: clear goal and rating feedback; named complaints, so the player knows what to fix ([[Next Steps]]); a build menu that reads well; a clear sign when the resort or its lifts are closed and the player has to open them (the user's playtest started with everything closed and nothing said so; no tutorial needed).
10. **Graphics polish.** [[Terrain Realism]] leftovers, [[Hiding the Grid]], and Kirkwood's frame rate (about 35 fps against Boreal's 42); models, and procedural roads and buildings (from the playtest; the skiers' tracks already look good).
11. **Fast-forward performance.** With only 50 guests, the top speeds were jumpy in the playtest; players will want to skip days quickly to earn money, so the sim needs to keep up at high speed. Done in [[Fast-Forward Performance]] (2026-10-08).
12. **Traffic you can see.** Traffic works (on the playtest save every car drove in from a map-edge entry, the first at 6:18), but about 15 cars an hour each spend some 5 clock minutes on the road, a second or two at high speed, so it read as cars appearing in the lot. Traffic is a big part of the game: busier, slower, or more visible arrivals.
13. **Menus and settings.** Start screen, save, load, and quit, and a settings check. Done 2026-10-13: Settings opens from the start menu (it was a stub) and gains full screen; the start menu has a backdrop and title, and the in-game menu a Resume. Left: the save and load screens' look.
14. **The demo build.** A separate build without the editor, the other scenarios, debug flags, testbeds, or the left-out features; packaged for Steam ([[Release]]). Planned 2026-10-12:
    1. **Scaffolding.** A `demo` build tag and `internal/edition` (`edition.Demo`, a constant, so guarded branches compile away). `make demo` builds with `-tags demo -trimpath -ldflags "-s -w"` into `build/demo/`, copying only the assets the demo uses (`boreal.save`, and models without the gondola, helicopter, snow gun, bar or tent). The demo keeps its saves in a folder of their own, so full-game saves never load into it.
    2. **Dev entry points out.** `main.go`'s dev flags (testbeds, headless, screenshots, profiling, the pprof endpoint, live queries) go in a `!demo` file; so do the scenario editor and terrain import (with `internal/geo`), the testbed menu, the debug console, `sim`'s testbeds, headless runner and query server, and the start menu's Editor and Testbeds buttons and the escape menu's "Save as Scenario".
    3. **Features out**, one at a time: snow gun, heli, gondola, 6-pack, bar, tent, rental shop (guests still arrive with skis rented in town), and land buying (parcel ownership stays). Each feature's code moves to `!demo` files; shared switches are guarded by `edition.Demo`, and any struct field the feature adds (including save fields) moves into an embedded struct that's empty in the demo.
    4. **Audit.** `make demo-audit` lists the demo binary's symbols and strings and fails on any left-out name (Heli, Gondola, SnowGun, 6-Pack, Bar, Tent, Rental, Parcel buying, Editor, Testbed, pprof, …). Then play the demo build through stage 1 of the tutorial.

    Not decided: garble obfuscation (the user chose stripping, not obfuscation, for now).

    Done 2026-10-12, steps 1–4 except playing the demo build (`make demo`, `make demo-audit`: clean). What was learned: Go keeps every method's *name* in type metadata even when the method is dead, and a struct's field names whenever the type is used, so dead-code elimination alone isn't enough. Left-out code is plain functions behind `edition.Demo` guards, or `!demo` files with plain-function stubs. Left-out state sits in embedded structs with neutral names (`liftExtras`, `liftDataExtras`, `bigLifts`, `scenarioDev`). The snow gun's flag became a neutral `Building.On` (save key `on`). `Lift.IsHeli` became `Cableless`, constant-false in the demo. `Service.Available` and `LiftType.Available` are false for left-out services and lifts, and `Offers` and `SetTileService` refuse them, so a hand-edited save can't run a bar or rental shop in the demo. What stays: the left-out types' enum values, cost constants, the guest-side rental needs (guests still rent in town), `Building.RentalPrice`, and parcel ownership. `tools/` and tests that need testbeds are `!demo`. The tutorial play-through (`tools/tutorial`) gives identical days in both builds.

## Open questions

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
- 2026-10-08: Service staffing and quality added as step 3 ([[Service Quality]]).
- 2026-10-08: Balance (step 4): the Goals Test settles around a 0.60 rating after fixing "too easy" judging and meal timing ([[Moments]]); 0.70 needs either more on the ledger or a lower target. The scenario files carry no goals yet.
- 2026-10-09: Balance (step 4) may move to [[Star Ratings]] and [[Scoreless Rating]]: Boreal's goal at 3★ (every need met), no score to calibrate. Measured on the Goals Test: 31–42% of guests at 3★; rentals, bored novices, and thirst hold the rest.
- 2026-10-09: The rating is now stars from moments ([[Scoreless Rating]]); the Goals Test reaches 3.02★ after one day and 3.12★ after two, against a 3★ goal. What holds guests below: thirst, bored novices, no lounge.
- 2026-10-10: The magic carpet is out of the demo for now (the user).
- 2026-10-12: Boreal only; the build menu limited (no snow gun, gondola, heli, 6-pack, bar, tent, rentals or land buying), and the left-out features' code compiled out of the demo binary (the user). Step 14 planned, and built the same day: `make demo` and a clean `make demo-audit`.
- 2026-10-13: QA notes: settings from the start menu, events cut to helpful ones, Overview tabs and map views trimmed, and the UI pass (steps 9 and 13).
- 2026-10-13: More QA: lift stations rebuilt so riders load under them, with a long terminal for detachables ([[Lifts]]); settings check boxes; the top bar's separator padded.
- 2026-10-13: Release builds for all three platforms from the Mac (the user): `make demo-mac` (a universal .app for macOS 11+, ad-hoc signed, assets in Resources), `make demo-cross` (Windows and Linux x86-64 in an x86-64 Debian 11 container, `tools/dist/Dockerfile`, via the lima Docker VM; the Linux binary needs glibc 2.27+), `make demo-all` for both; each audited, zipped into `build/dist/`. The user is weighing a Steam demo over self-release; Steam takes the same per-platform builds as depots.
- 2026-10-13: The user paid the Steam Direct fee; tax form approval pending. The demo is on hold until then. Next on Steam: the base game's store page (public as Coming Soon for two weeks before anything is playable), then the demo app with the three builds as depots; Next Fest February 2027 is the likely target.

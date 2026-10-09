---
title: Lift Unloading
kind: plan
status: done
---

# Lift Unloading

Getting off a chair the way real skiers do: stand up at the unload point, ski straight ahead down the ramp onto the apron, peel left or right by seat, and only then turn downhill. Unloading becomes its own guest state, so it can be drawn as an unloading animation and so falls there come from a deliberate roll rather than the balance model. It also fixes the lift-top step behind most falls ([[Next Steps]], the Falls item). Part of [[Lifts]]; touches [[Skiing]] and [[Moments]].

## Why

Diagnosed on 2026-10-07 on the user's Boreal save. Every fall, 392 in two days, was a beginner 5–20 m from the top post, within 15 s of unloading, on ground of about 20°. Two things combine.

- **Unloading is a teleport.** When a chair's progress crosses 0.5 (the top post), `tickLifts` (`internal/sim/simulation.go`) places every rider on the post's centre point, `lift.Top`. That's the top cell, which is marked impassable and was 29° on Boreal. All riders land there whatever their seat. Each gets speed 0 and a heading straight at their next plan target, usually back down the lift line. Balance is only reset if it's below 0.5. Riders never use the apron, which lies beyond the post on the bullwheel side; they set off from a standstill down the cable side.
- **The top apron leaves a step on the cable side.** `scene/lift_apron.go` (049eb95) levels the apron to the natural ground 4 m beyond the post. On Boreal that's uphill, so the apron sits 2–4 m above the station's ground. `apronWeight` is 0 on the cable side of the post with no bank, so there's a drop of 18–30° within one 5 m cell, exactly where the teleported riders head. A beginner (10° comfort slope) loses balance at about 0.8/s on 20° and falls in under 2 s.

## Decisions

From the user, 2026-10-07:

- **Riders ski straight ahead off the chair, then bend away from the lift by seat, then turn downhill**, as real unloading does.
- **Unloading is its own state.** It can be drawn as an unloading animation and decide when guests fall, instead of the balance model.
- **Falls getting off beginner lifts are real** and should stay, at a believable rate that the unloading state controls.

Confirmed by the user, 2026-10-07:

- **Bank the apron's cable side** like its other sides, and cap every apron bank at a beginner-safe grade (about 12°), rather than levelling the apron to the station's ground (which shrinks the step but doesn't remove it on steep hills).

## The model

**The unload point.** Riders get off where the chair reaches the station, with each rider's start position being their seat's ground position (`seatWorldPos` dropped to the snow) rather than the post. Seats come from the chair mesh's slots (`world.SlotsFor`). Their chair-local X gives each rider a side: left seats negative, right seats positive. On a quad or six-pack, the outer seats peel harder than the inner ones.

**The unloading state.** `Guest.Unload` holds the lift, the side and peel strength from the seat, the start position, and elapsed time. `world.Activity` reports "Unloading". `tickGuests` dispatches it ahead of normal skiing, and the planner and `tickSkier` leave the guest alone until it ends. It runs three phases on a scripted path at ramp speed (about 2–3 m/s, a gentle glide):

1. **Stand and glide.** Straight ahead along the lift axis, away from the cable and onto the apron, for about 4 m. The rider rises from seat height to standing over the first half second.
2. **Peel off.** An arc away from the lift centreline toward the rider's side, about 6 m long, clearing the unload lane for the next chair.
3. **Hand off.** At the arc's end the guest returns to normal skiing with the arc's speed and heading. Their plan target then turns them downhill around the side of the station, not over the post.

Balance is held at full while unloading, and set to 1 when it ends.

**Falls getting off.** One roll per rider when unloading starts, by skill and lift type. Beginners fall most, detachable chairs (which slow down at the terminal) least, and fixed-grip doubles and quads most. A rough start is about 3% for beginners on a fixed-grip chair, under 1% for intermediates, and near 0 for advanced; tune by eye. A rider who falls stops partway through phase 1 or 2 and lies in the unload zone for the usual fall time. That's a new event, "fell getting off the lift" (about −0.05, more embarrassing than painful, no injury roll), and they finish the peel-off once up. The next chair's riders steer around them. Later: lift attendants stop or slow the lift for a fallen rider ([[Next Steps]], Staff and [[Lift Operations]]).

**The apron.** `apronWeight` gets a bank on the cable side, and every bank is capped so its grade stays under about 12°: the bank is lengthened, or the apron's target pulled toward the station's ground, whichever keeps the grade under the cap. Saved worlds keep their old ground until the lift is re-placed, so a one-off repair re-carves every lift's aprons (for Boreal's bundled scenario and the user's save).

## Steps

1. Done: **Apron banks.** `apronWeight` banks every side, the cable side included. `fitApron` picks each station's bank (8–24 m) and apron height (up to 4 m from its target, in half-metre steps) so that no cell the carve touches is steeper than `liftApronMaxGrade`. It measures slope as the terrain does (gradient magnitude, `apronExcess`); if nothing fits, it takes the least-steep try. Changed from the plan: the cap is 15°, not 12°. A flat apron cut into an ordinary 11° hill can't keep its banks under 12°. 15° is where a beginner's slope drain matches their balance recovery. The repair is the existing debug-console `regrade` (now `scene.RegradeEmbankments`) plus `tools/regrade in.save out.save`, which changes only the ground. Applied to `assets/scenarios/boreal.save`, and to the user's save as a new file, `save-2026-10-06-2213-regraded.save`, leaving the original alone.

   Checked on the user's Boreal save. The steepest cell within 20 m of the top post went from 29.6° (26 cells over 12.5°) to 17.2° (5 cells over 15.5°), and those 5 are on the natural hill. Falls within 25 m of the top post in a headless day went from 229 to 0, with 0 elsewhere both times (same seed, about 110 departures).
2. Done: **Unloading state.** `Guest.Unload` (`world.Unloading`), `startUnloading` and `tickUnloading` in `internal/sim/unloading.go`, and the "Unloading" activity. The teleport to `lift.Top` is gone. Riders start on the snow under their seat, facing up the lift line. Their side comes from the seat slot's chair-local Z, or the seat index when no slots are registered (headless). They glide 4 m straight, then a 6 m arc turning 30–70° to their side, then hand off to skiing at 2.5 m/s. Checked headless on the regraded Boreal save (one day): all 187 unloads finished, on average 7.5 m beyond the post and 6.2 m to the side. No tick was inside the post's cell, and the longest unload took 5 s against a chair every 12.4 s. A lone rider always takes the first seat, so sides split about 58/42.
3. Done: **Unload falls.** `unloadFallChance` by skill and lift type (beginners 3% on fixed-grip chairs and 1% on detachables; intermediates 0.5% / 0.2%; advanced 0.1% / 0; gondolas none). The fall lands between 1 m and the middle of the arc, gives "I fell getting off the lift!" (−0.05), and recovers through `tickFallen`, after which the glide finishes. Riders behind peel off early when someone is down within 4 m ahead. Checked headless over two days: 9 falls in 306 unloads (2.9%; about three-quarters of riders are beginners on a fixed-grip double), and no falls anywhere else.
4. Done, partly: **Drawing it.** There are no skier poses yet, so the rider eases from the seat's height to the snow over the first 1.25 m (`Unloading.UnloadLift`, read by the renderer). Screenshots of Boreal's top show the banked apron, but none caught a rider mid-unload: with about 28 guests on 24 chairs, most arrive empty. Left for the user to watch in play; a real stand-up pose waits on the animation approach (Rendering, in [[Next Steps]]).
5. Done: **Docs.** [[Lifts]] and [[Guests Spec]] (Unloading).

Each step builds with `go build` and `go vet` and is checked headless or by screenshot. No Go tests.

## Open questions

- Whether middle seats on a six-pack peel by seat or by where they're heading next.
- Loading at the base as the counterpart (the chair scooping riders from a load line, and missed chairs); a separate plan if wanted.
- Whether a fallen rider should stop the lift for everyone before lift attendants exist.
- Heli drop-offs keep their own unloading.

## Log

- 2026-10-07: Planned with the user after diagnosing falls at the top station.
- 2026-10-07: The user confirmed the apron fix (bank the cable side, cap the grade) and the ranking.
- 2026-10-07: Step 1: aprons banked on every side and fitted under 15°; Boreal's falls at the top post went from 229 a day to 0.
- 2026-10-07: Steps 2–5: the unloading state, unload falls, the stand-up drawing, docs. On the regraded Boreal save, the only falls are unloading falls (about 3% of rides).

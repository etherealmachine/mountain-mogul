---
title: Surface Lifts
kind: plan
status: in progress
---

# Surface Lifts

Lifts where riders keep their skis on the snow: the magic carpet, the rope tow, and the T-bar. They're cheap, short or simple, and they're how beginner areas work: a carpet by the ski school, a rope tow on the bunny hill, a T-bar up a wide open slope. Part of [[Lifts]]; wanted by [[Families]] (lessons), the novice boredom bug in [[Next Steps]], [[Kirkwood]] (two carpets and a T-bar in OpenStreetMap), [[Boreal]], and Portillo in [[Scenario Campaign]].

## Decisions

From the user, 2026-10-13:

- **All three types**, magic carpet first.
- **Anyone can ride.** No skill limit: a carpet is for beginners, but anyone may take it when it's the quickest way somewhere (from a lot up to the base, say).
- **Novices avoid rope tows and T-bars** (`liftAccessible`, `ai/goap/goal.go`); beginners ride them and fall off more often. Novices ride the carpet.
- **Wind doesn't stop them.** Wind holds in [[Lift Operations]] skip surface lifts, so they keep running when the chairs are held.
- **The carpet is always open**: a belt with side rails, no tunnel.
- Not in the [[Demo]]: like the 6-pack, behind `!demo`, left out of `Available()`, their models in `DEMO_NO_MODELS`, and their names in `tools/demo-audit.sh`.

## How it fits the lift code

A surface lift is a lift type on the chair loop the lifts already run (`tickLifts`, `loadChair`, `tickRiding`): each "chair" is a place on the belt, rope or T-bar, riding at ground height. Line waits and riders per hour come out of the chair count and loop time unchanged, lines and corrals work as they are (a carpet's corral rows are one wide), and boarding at the wrap point spaces riders out by construction. What's new is a few predicates on the type (`HasTowers`, `HasCable`, ground height in `CableHeightAt` and `ChairPos`) rather than touching every `Cableless` check, which stays heli-only.

## Steps

In order. Each step builds with `go build` and `go vet`, and is checked headless on the Boreal Goals Test and on screen.

1. Done: **The magic carpet.** `LiftMagicCarpet` (Lifts menu, full game only): one standing rider per place, places 2.5 m apart, 1 m/s, at `BeltHeight` (0.12 m) over the ground; no towers or cable; `beltStation` ends, no station house: the belt's end plate and a control post with stop and start buttons (`lib/carpet_kit.py`; the top end, `magic_carpet_top.py`, is the base mirrored, since the belt runs on one side of the line); one operator at the base's post; up to 120 m (about 400 ft, the long end of real carpets) and 12° over any 10 m (`liftLineProblem`: the ghost, belt and all, turns red and the hint says why). $90k plus $600 a metre, $200 a day plus the operator. Riders glide straight off over the top's end plate for 3 m (no peel, no falls). A 4 m tree corridor and small aprons.
2. Done: **Short runs count.** A run off a lift's top counts if it drops half the lift's rise, between 3 and 20 m (`Lift.MinRunDrop`, used by the trail graph and the SkiTo* actions). On the "Magic carpet, bunny slope, novice skiers" testbed (8 m of rise) novices queue, ride, ski the green and lap. Left: the novice boredom bug on Boreal with a carpet beside Kiss A Bear (step 6).
3. Done: **The carpet's look.** The belt between low side walls, drawn on the snow and rebuilt every 30 frames so storms don't bury it (`generateBeltMesh`, `syncBelts`); the end models (`magic_carpet_end.py`, `magic_carpet_top.py`); riders standing in their ski stance.
4. **The rope tow.** A second ground-riding type: a rope on low posts, riders gripping it and sliding up a packed track on their skis. Longer and steeper than a carpet (up to about 300 m and 25°). Novices don't ride it. Riders can fall off: a chance by skill on each ride, after which they let go, get up beside the track, and ski down or rejoin the line, annoyed ("fell off the rope tow"). The track packs the snow under it.
5. **The T-bar.** Towers and a cable like a chair, with T-shaped hangers pulling two riders side by side up a track (one if riding alone). Long and steep (up to about 1,500 m). Falls as for the rope tow, more likely for beginners. A model for the towers, the bar and the stations.
6. **Balance and placement.** Costs and running costs for the three types against chairs; Boreal with a carpet by the base and Kirkwood with its OpenStreetMap carpets and T-bar placed; measure novice boredom, falls on tows, and line waits.

## Open questions

- Do they close in heavy snow like the chairs (the snow hold at the base), or keep running?

## Log

- 2026-10-13: Planned with the user: all three surface lifts, the carpet first and always open, anyone may ride.
- 2026-10-13: Novices avoid rope tows and T-bars; surface lifts keep running in wind (the user).
- 2026-10-13: Steps 1–3 built: the magic carpet, short runs counting, its belt and end model. Found on the way: testbed guests built with `goapSkierAt` start with no energy or day ticket, so they plan to go home at once; the carpet testbed fills both.
- 2026-10-13: No station house on the carpet (the user); its operator stands by the belt. The illegal ghost looked like it vanished, since the belt line was never tinted and drew pale grey on snow: ghost cables and belts now take the placement tint, and the hint names the problem while hovering.
- 2026-10-13: A control post with stop and start buttons at each end (the user). The top end was the base model turned to face down the line, which put its plate on the far side from the belt; it's now its own mirrored model.

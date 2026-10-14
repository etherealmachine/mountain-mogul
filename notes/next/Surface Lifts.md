---
title: Surface Lifts
kind: plan
status: shipped
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
4. Done: **The rope tow.** `LiftRopeTow` (Lifts menu, full game only): one rider a place, places 8 m apart, 2.5 m/s, on the snow; the rope `RopeGap` off the line each side at `RopeHeight` on posts about every 25 m (`RopePostXZs`); a flat bullwheel at each end (`rope_tow_base.py` with the motor and the operator's control post, `rope_tow_top.py` with a stop post, parts in `lib/rope_tow_kit.py`); up to 300 m and 25°. $40k plus $120 a metre, $150 a day plus the operator. Novices won't ride a tow (`liftAccessible`). Riders fall off by skill tier on 20%, 4% and 0.5% of rides (beginner, intermediate, advanced), never within 4 m of either end (`sim/tows.go`): they go down beside the track with no plan, annoyed ("I fell off the rope tow!"), and plan afresh once up. Riders pack the track (`packUnderfoot`, wear without moguls). Riders lean on their skis with both hands on the rope. The `Belt()` and `Tow()` predicates split the two under `Surface()`. Testbed: "Rope tow, intermediate and beginner skiers".
5. Done: **The T-bar.** `LiftTBar` (Lifts menu, full game only): two riders a hanger, hangers 15 m apart, 3 m/s, up to 1,500 m and 30°. The cable runs `TBarCableHeight` (4.5 m) over the snow the whole way on towers (`tbar_tower.py`, `TowerMesh`), so each place on the loop is a hanger hanging from it like a chair, and the riders' feet, its seat slots, are on the snow (`tbar_hanger.py`; an empty hanger is drawn reeled in, `tbar_hanger_up.py`). The ends are a flat bullwheel on a boom off a mast, with the motor at the base and the tension weight at the top, and a control post at each (`lib/tbar_kit.py`); an operator at each end, outside the riders. $250k plus $140 a metre, $700 a day plus two operators. Novices won't ride it; falls are per ride by skill tier, spread along the track: 25%, 5%, 1% (the rope tow's are 20%, 4%, 0.5%). `Tow()` now covers both tows (novices, falls, packing); the rope tow's own parts are `Rope()`, the T-bar's `Hangers()`. Testbed: "T-bar, intermediate and beginner skiers".
6. **Balance and placement.** Costs and running costs for the three types against chairs; Boreal with a carpet by the base and Kirkwood with its OpenStreetMap carpets and T-bar placed; measure novice boredom, falls on tows, and line waits.

## Open questions

- Do they close in heavy snow like the chairs (the snow hold at the base), or keep running?

## Left over

Moved from [[Next Steps]] on 2026-10-14, when it was cut down to new features.

- [[Surface Lifts]] leftovers: place a magic carpet by Boreal's base, and Kirkwood's carpets and T-bar from OpenStreetMap; then measure novice boredom (see Bugs), falls on tows, and line waits. Their prices are under Economy.

## Log

- 2026-10-13: Planned with the user: all three surface lifts, the carpet first and always open, anyone may ride.
- 2026-10-13: Novices avoid rope tows and T-bars; surface lifts keep running in wind (the user).
- 2026-10-13: Steps 1–3 built: the magic carpet, short runs counting, its belt and end model. Found on the way: testbed guests built with `goapSkierAt` start with no energy or day ticket, so they plan to go home at once; the carpet testbed fills both.
- 2026-10-13: No station house on the carpet (the user); its operator stands by the belt. The illegal ghost looked like it vanished, since the belt line was never tinted and drew pale grey on snow: ghost cables and belts now take the placement tint, and the hint names the problem while hovering.
- 2026-10-13: A control post with stop and start buttons at each end (the user). The top end was the base model turned to face down the line, which put its plate on the far side from the belt; it's now its own mirrored model.
- 2026-10-13: Step 4 built: the rope tow. On its testbed beginners fall off about one ride in four and come back to the line; the novice never rides. Every guest there goes home bored after about one run, as on the carpet's testbed: one short run doesn't hold their tastes for long. Left for step 6 with the novice boredom check.
- 2026-10-13: Step 5 built: the T-bar. Falls changed from a chance per 100 m to a chance per ride spread along the track, since on a 400 m T-bar the per-metre chance had beginners coming off most rides. Headless (no renderer) riders sit at the cable, as chair riders do: seat slots come from the models.
- 2026-10-14: Done (the user). Step 6's balance and placement moved to [[Next Steps]]: balance with the economy's, placing them in Boreal and Kirkwood under Lifts.

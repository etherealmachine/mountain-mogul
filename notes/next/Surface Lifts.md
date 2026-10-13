---
title: Surface Lifts
kind: plan
status: planned
---

# Surface Lifts

Lifts where riders keep their skis on the snow: the magic carpet, the rope tow, and the T-bar. They're cheap, short or simple, and they're how beginner areas work: a carpet by the ski school, a rope tow on the bunny hill, a T-bar up a wide open slope. Part of [[Lifts]]; wanted by [[Families]] (lessons), the novice boredom bug in [[Next Steps]], [[Kirkwood]] (two carpets and a T-bar in OpenStreetMap), [[Boreal]], and Portillo in [[Scenario Campaign]].

## Decisions

From the user, 2026-10-13:

- **All three types**, magic carpet first.
- **Anyone can ride.** No skill limit: a carpet is for beginners, but anyone may take it when it's the quickest way somewhere (from a lot up to the base, say).
- **The carpet is always open**: a belt with side rails, no tunnel.
- Not in the [[Demo]]: like the 6-pack, behind `!demo`, left out of `Available()`, their models in `DEMO_NO_MODELS`, and their names in `tools/demo-audit.sh`.

## How it fits the lift code

A surface lift is a lift type on the chair loop the lifts already run (`tickLifts`, `loadChair`, `tickRiding`): each "chair" is a place on the belt, rope or T-bar, riding at ground height. Line waits and riders per hour come out of the chair count and loop time unchanged, lines and corrals work as they are (a carpet's corral rows are one wide), and boarding at the wrap point spaces riders out by construction. What's new is a few predicates on the type (`HasTowers`, `HasCable`, ground height in `CableHeightAt` and `ChairPos`) rather than touching every `Cableless` check, which stays heli-only.

## Steps

In order. Each step builds with `go build` and `go vet`, and is checked headless on the Boreal Goals Test and on screen.

1. **The magic carpet.** `LiftMagicCarpet` added at the end of the type enum (old saves keep their values). One rider per place, places about 3 m apart (a per-type spacing replacing `ChairSpacingM` in `PlaceLift`), belt speed about 1 m/s, riding at ground height. No towers or cable. A small station geometry (head and tail housings, a short flat) so `MinLiftLength` is short. One operator at the base (a per-type operator count in `Headcount` and `StaffPosts`). Placement checks in the lift tool, per type: a carpet runs up to about 120 m and up to about 12°. Cheap to build and run (numbers set in the balance pass, step 6). Riders stand, and at the top step straight off onto a short run-out with no peel; no unload falls.
2. **Short runs count.** Today a trail edge needs 20 m of drop (`trailMinDescentMeters`, `world/trail_graph.go`; `minDescentMeters` in `ai/goap/action.go`), so a carpet with 10 m of rise serves no runs and nobody rides it. Edges from a short lift's top count with a drop scaled to the lift's own rise (at least a few metres). Then check that novices pick the carpet, the "No runs off" hint stays quiet, and the novice boredom bug improves with a carpet beside Kiss A Bear.
3. **The carpet's look.** A belt draped on the ground between the housings, with side rails, as a build123d model ([[Model Pipeline]]); riders drawn standing, not seated; the return run hidden.
4. **The rope tow.** A second ground-riding type: a rope on low posts, riders gripping it and sliding up a packed track on their skis. Longer and steeper than a carpet (up to about 300 m and 25°). Riders can fall off: a chance by skill on each ride, after which they let go, get up beside the track, and ski down or rejoin the line, annoyed ("fell off the rope tow"). The track packs the snow under it.
5. **The T-bar.** Towers and a cable like a chair, with T-shaped hangers pulling two riders side by side up a track (one if riding alone). Long and steep (up to about 1,500 m). Falls as for the rope tow, more likely for beginners. A model for the towers, the bar and the stations.
6. **Balance and placement.** Costs and running costs for the three types against chairs; Boreal with a carpet by the base and Kirkwood with its OpenStreetMap carpets and T-bar placed; measure novice boredom, falls on tows, and line waits.

## Open questions

- Should the T-bar and rope tow be harder to ride for some guests (a fall chance by skill only, or do novices avoid them)?
- Do carpets or tows close in high wind or heavy snow, or keep running when the chairs are on hold?

## Log

- 2026-10-13: Planned with the user: all three surface lifts, the carpet first and always open, anyone may ride.

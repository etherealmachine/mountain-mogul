---
title: Lifts
kind: system
status: partial
---

# Lifts

Lift types run from a fixed double through the fixed triple, fixed quad, high-speed quad, high-speed six-pack, and gondola, plus a heli-ski "lift" with no cable. Type sets seats per chair and cable speed. A lift can be upgraded to a better type on the same line for the price difference. Lift lines can be split into lanes. Heli is the only lift that charges per ride; everything else is covered by [[Tickets]].

[[GOAP]] plans every ride: walk or ski to the base, join the line, ride to the top. Lines drain [[Patience]]. A guest won't join a line past 20 people, and if every lift they can use is over that cap they leave. A lift whose base has no [[Snow]] goes on hold until snow returns. Total lift capacity sets how crowded [[Demand]] thinks the resort is.

The [[Trails]] off a lift's top decide which skill levels it serves.

At the top, riders stand up from their seats and glide about 4 m straight off the ramp onto the station's flat apron, away from the cable. Then they peel left or right by where they sat (outer seats harder), and only then ski toward their next target. Around each station's apron the earthwork cuts and fills only ground steeper than 15° from its edge, and doesn't make natural slopes steeper ([[Terrain]]). Some riders fall getting off: about 3% of beginners on fixed-grip chairs and 1% on detachables, fewer for better skiers, none on gondolas.

Not built yet, from [[Vision]] and [[Next Steps]]: wear, breakdowns, wind holds, downloading, and refusing overlapping lifts ([[Lift Operations]]); lift attendants as people on the map (each lift already pays for two a day); partly filled chairs; and lift lines that wrap around buildings.

Code: `internal/world/lift.go`. Upgrades: `UpgradeLift` in `internal/world/world.go`.

## Log

- 2026-10-01: Lift types, upgrades, lanes, the queue cap, snow holds, and heli are in. Wear, wind holds, and attendants are not.
- 2026-10-06: Stations get a flat apron in front of the post, on the bullwheel side where skiers queue and ski off (about 8 × 12 m, flat in the lidar too), rather than a raised pad; nothing on the cable side changes ([[Terrain]]).
- 2026-10-07: Unloading is a teleport to the top post, and the top apron has an unbanked step on the cable side; together they cause nearly all falls. Planned as [[Lift Unloading]].
- 2026-10-07: Unloading from the seat with a glide and peel-off, unload falls by skill and lift type, and aprons banked on every side under 15° ([[Lift Unloading]]).
- 2026-10-07: Aprons grade only what they must; the old fitter could dig a pit into a knoll ([[Terrain]]).
- 2026-10-08: The chairlift station model is a real terminal: a spoked bullwheel centred where the cables end, under a hood on a portal frame, with a drive box and an operator's hut.
- 2026-10-08: Towers are tapered round poles with a crossarm above the cable, sheave trains carrying it, and a ladder. Chairs have a grip, hanger, frame, padded seat and backrest in a colour per type (red double, blue quad, navy 6-pack), armrests, a raised restraint bar, and a footrest; they're no longer tinted grey when empty.
- 2026-10-08: Chairs hang lower: the footrest is the station cable height (3.65 m) below the cable, so riders' skis are on the snow at the bullwheel and the seat rides about 3.1 m under the cable.
- 2026-10-08: Fixed triple: three seats at 2.5 m/s, green seat, $550k stations, $175/m, $350/day; doubles upgrade to a triple or a quad, triples to a quad.
- 2026-10-08: Getting off: a longer run-out (5 m straight, clear of the station, then a 10 m arc) that peels toward the trail the guest is about to ski (20 m down its centre line from where it passes the top), turning 25–100°, or to the seat's side when the trail is dead ahead; hand-over heading off target down from 81° to 60° on Boreal. Stations' portal legs and huts are steering hazards like towers (`Lift.StationXZs`), and walkers sidestep towers and station parts on the way; samples within 1 m of top-station parts fell from about 600 to 120 a day, base legs from 1,560 to 38. The base huts stand in the lift lines (Bugs, [[Next Steps]]).

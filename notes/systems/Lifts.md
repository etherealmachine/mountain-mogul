---
title: Lifts
kind: system
status: partial
---

# Lifts

Lift types run from a fixed double through the fixed quad, high-speed quad, high-speed six-pack, and gondola, plus a heli-ski "lift" with no cable. Type sets seats per chair and cable speed. A lift can be upgraded to a better type on the same line for the price difference. Lift lines can be split into lanes. Heli is the only lift that charges per ride; everything else is covered by [[Tickets]].

[[GOAP]] plans every ride: walk or ski to the base, join the line, ride to the top. Lines drain [[Patience]]. A guest won't join a line past 20 people, and if every lift they can use is over that cap they leave. A lift whose base has no [[Snow]] goes on hold until snow returns. Total lift capacity sets how crowded [[Demand]] thinks the resort is.

The [[Trails]] off a lift's top decide which skill levels it serves.

Not built yet, from [[Vision]] and [[Next Steps]]: wear, breakdowns, wind holds, downloading, and refusing overlapping lifts ([[Lift Operations]]); lift attendants as people on the map (each lift already pays for two a day); partly filled chairs; and lift lines that wrap around buildings.

Code: `internal/world/lift.go`. Upgrades: `UpgradeLift` in `internal/world/world.go`.

## Log

- 2026-10-01: Lift types, upgrades, lanes, the queue cap, snow holds, and heli are in. Wear, wind holds, and attendants are not.

---
title: Thirst
kind: stat
status: shipped
---

# Thirst

A countdown on each guest, from full at 1 down to leaving at 0. [[GOAP]] reads it. Spawned at random in `[0.5, 1)`. Drains while skiing (five clock hours of it from full to empty), faster with altitude (+40% at 2,000 m) and with exertion on terrain above the guest's skill; a quarter as fast riding, in a line, or walking; not at all while being served.

Below 0.55 the thirst need (`FulfillNeed`) outweighs skiing (1.05 and up, as hunger's does) and sends them to a [[Bar]] or a [[Food Court]] for a drink; a meal fills thirst too. Nearly empty, that goal is tried before `GoHome`. Below 0.15 the thirsty condition starts; held past the grace period it's a letdown in [[Moments]], but the grace clock waits while the guest is on the way to a drink. A drink, or free water where it's offered, sets thirst back to 1.

Leaving because of thirst does not dock [[Moments]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, the thirst goal, the thought, and the drink restore are in.
- 2026-10-07: Low thirst is now a condition that pulls satisfaction down ([[Satisfaction Rework]]).
- 2026-10-07: Food courts pour drinks and meals fill thirst; the thirst goal outweighs skiing below 0.25, as hunger's does.
- 2026-10-07: Thirst is a need with an urgency; the goal is `FulfillNeed` ([[Service Improvements]] step 1).
- 2026-10-09: Presses at 0.55 (was 0.4), drains a quarter as fast off the snow, and altitude counts for less; free water. Balancing Christmas on the [[Season Calendar]].

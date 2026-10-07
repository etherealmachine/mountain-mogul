---
title: Thirst
kind: stat
status: shipped
---

# Thirst

A countdown on each guest, from full at 1 down to leaving at 0. [[GOAP]] reads it. Spawned at random in `[0.5, 1)`. Drains while skiing, faster with altitude and with exertion on terrain above the guest's skill.

Below 0.25 the `RelieveThirst` goal outweighs skiing (1.05 and up, as hunger's does) and sends them to a [[Bar]] or a [[Food Court]] for a drink; a meal fills thirst too. Nearly empty, that goal is tried before `GoHome`. Below 0.15 the thirsty condition starts: they think about wanting a drink once, and it pulls [[Satisfaction]] down until thirst recovers past 0.25. A drink sets thirst back to 1.

Leaving because of thirst does not dock [[Satisfaction]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, the thirst goal, the thought, and the drink restore are in.
- 2026-10-07: Low thirst is now a condition that pulls satisfaction down ([[Satisfaction Rework]]).
- 2026-10-07: Food courts pour drinks and meals fill thirst; the thirst goal outweighs skiing below 0.25, as hunger's does.

---
title: Hunger
kind: stat
status: shipped
---

# Hunger

A countdown on each guest, from full at 1 down to leaving at 0. [[GOAP]] reads it. Spawned at random in `[0.5, 1)`. Drains on a fixed clock while the guest is skiing, five clock hours from full to empty, and a quarter as fast riding, in a line, or walking; not at all while being served. Presses at 0.55.

Below 0.25 the hunger need (`FulfillNeed`) outweighs another lap and the guest heads for a [[Food Court]]. Below 0.15 the hungry condition starts: they think about wanting a meal once, and it pulls [[Moments]] down until hunger recovers past 0.25. Below 0.05 `GoHome` wins if no meal is available. A meal sets hunger back to 1.

Leaving because of hunger is a normal end to the day. It does not dock [[Moments]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, the hunger goal, the thought, and the meal restore are in.
- 2026-10-07: Low hunger is now a condition that pulls satisfaction down ([[Satisfaction Rework]]).
- 2026-10-07: Hunger is a need with an urgency; the goal is `FulfillNeed` ([[Service Improvements]] step 1).
- 2026-10-09: Presses at 0.55 (was 0.4) and drains a quarter as fast off the snow ([[Season Calendar]]).

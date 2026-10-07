---
title: Hunger
kind: stat
status: shipped
---

# Hunger

A countdown on each guest, from full at 1 down to leaving at 0. [[GOAP]] reads it. Spawned at random in `[0.5, 1)`. Drains on a fixed clock while the guest is skiing, about five hours of sim time from full to empty.

Below 0.25 the `RelieveHunger` goal outweighs another lap and the guest heads for a [[Food Court]]. Below 0.15 the hungry condition starts: they think about wanting a meal once, and it pulls [[Satisfaction]] down until hunger recovers past 0.25. Below 0.05 `GoHome` wins if no meal is available. A meal sets hunger back to 1.

Leaving because of hunger is a normal end to the day. It does not dock [[Satisfaction]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, the hunger goal, the thought, and the meal restore are in.
- 2026-10-07: Low hunger is now a condition that pulls satisfaction down ([[Satisfaction Rework]]).

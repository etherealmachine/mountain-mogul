---
title: Satisfaction
kind: stat
status: shipped
---

# Satisfaction

The session score, from 0 to 1. Starts at 0.6. [[GOAP]] does not read it. When the guest leaves, this number is what moves the resort rating in [[Demand]].

Every tick, for every guest on the mountain, it drifts toward a target. The target is 0.5, plus the grooming pull while skiing, plus the pull of each active condition: hungry, thirsty, sick of waiting, or no lodge to rest in pull it down; glades or trees pull it up or down by trait. It jumps on events: a fall, tree hit, or injury in [[Skiing]] or an [[Avalanche]], being left by [[Ski Patrol]], a long line in the [[Lifts]], or a run on corduroy. Every amount lives in one table, `ai.Effects`, and every change is reported by a thought. A condition's thought is added once when it starts.

No [[Amenities]] write this stat directly yet. They change it by ending conditions (a meal ends hungry, a rest ends sick of waiting) and by keeping the guest on the mountain.

Older docs and the F4 debug panel call this Fun. In the sim the field is `Satisfaction`.

Being rebuilt by [[Satisfaction Rework]] (steps 1–3 done). Still to come: departure reasons, a daily-average rating, and service events.

Spec: [[Guests Spec]], the thoughts table (out of date: it lists a first-ride bonus and an off-piste thought that don't exist).

## Log

- 2026-10-01: Drift, the thought catalogue, and the rating average are in.
- 2026-10-07: Corrected: there is no first-ride bonus. Rework planned in [[Satisfaction Rework]].
- 2026-10-07: One effects table, condition thoughts, and mood in every activity with needs pulling it down ([[Satisfaction Rework]] steps 1–3).

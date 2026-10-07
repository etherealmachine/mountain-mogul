---
title: Satisfaction
kind: stat
status: shipped
---

# Satisfaction

The guest's mood for the visit, from 0 to 1. Starts at the baseline, 0.5. [[GOAP]] does not read it. When the guest leaves, it counts toward the day's average, which becomes the resort rating in [[Demand]] at midnight.

Every tick, for every guest on the mountain, it drifts toward a target: the guest's baseline, plus the grooming pull while skiing, plus the pull of each active condition. The baseline starts at 0.5 and is the guest's memory of the day. A great run raises it, by less for each repeat on the same trail and, more gently, off the same lift. Injuries, being left by patrol, slow patrol, an avalanche, and runs that were too much lower it ([[Mood Baseline]]). Hungry, thirsty, sick of waiting, no lodge to rest in, and runs that are too easy pull it down; glades or trees pull it up or down by trait. It jumps on events: falls, tree hits, injuries, an [[Avalanche]], how fast [[Ski Patrol]] arrived, long lines at the [[Lifts]], a meal, a drink, a rest, and the verdict on each run (great, too hard, crowded, perfect corduroy). Each run is judged at the bottom from what the guest actually skied: time on each trail difficulty, steepness, crowding, vertical, and falls. Every amount lives in one table, `ai.Effects`, and every change is reported by a thought.

Each departure records one reason, shown in the "Why guests left" chart. Tired, hungry, thirsty, done, or closing time is an ordinary end to the day.

Older docs and the F4 debug panel call this Fun. In the sim the field is `Satisfaction`.

Built by [[Satisfaction Rework]]; [[Snow Tastes]] adds per-guest snow tastes.

Spec: [[Guests Spec]], Satisfaction, Rating, and Thoughts.

## Log

- 2026-10-01: Drift, the thought catalogue, and the rating average are in.
- 2026-10-07: Corrected: there is no first-ride bonus. Rework planned in [[Satisfaction Rework]].
- 2026-10-07: One effects table, condition thoughts, and mood in every activity with needs pulling it down ([[Satisfaction Rework]] steps 1–3).
- 2026-10-07: Departure reasons, the daily-average rating, service and patrol events, and run verdicts from a per-run summary ([[Satisfaction Rework]] steps 4–7).
- 2026-10-07: A per-guest baseline that experiences move ([[Mood Baseline]]); mood is now saved for guests on the mountain.

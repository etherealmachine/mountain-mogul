---
title: Satisfaction
kind: stat
status: shipped
---

# Satisfaction

The guest's score for the day, from 0 to 1: a ledger that starts at 0.5 and never drifts back. [[GOAP]] does not read it. When the guest's car leaves the map, the score (with how they felt driving away) counts toward the day's average, which becomes the resort rating in [[Demand]] at midnight.

Events add to it once and for good, and conditions cost it by the clock hour while they last. A great run adds, by less for each repeat on the same trail and, more gently, off the same lift. Being hungry, thirsty, sick of waiting, without a lodge to rest in, or on runs too easy for them costs a little every clock hour, and each that's still on as they drive away costs its hourly amount once more. Events: falls, tree hits, injuries, an [[Avalanche]], how fast [[Ski Patrol]] arrived, long lines at the [[Lifts]], a meal, a drink, a rest, and the verdict on each run (great, with corduroy as a reason, too hard, crowded). Each run is judged at the bottom from what the guest actually skied: time on each trail difficulty, steepness, crowding, vertical, and falls. Every amount lives in one table, `ai.Effects`, and every change is reported by a thought.

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
- 2026-10-07: Snow underfoot against tastes replaces the grooming and glade pulls ([[Snow Tastes]] step 2).
- 2026-10-07: A ledger: events add once, conditions cost by the clock hour, nothing drifts back; recorded when the car leaves the map, with each condition still on costing once more. Replaces the baseline and drift ([[Mood Baseline]]).
- 2026-10-07: Boredom ("skied this place to death", "nothing here is my kind of skiing") costs by the clock hour and sends the guest home; crowded runs cost as much as a guest dislikes crowds ([[Snow Tastes]] step 5).

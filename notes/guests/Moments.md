---
title: Moments
kind: stat
status: shipped
---

# Moments

How a guest's day is judged: a collection of moments, and the star rating they leave from them ([[Scoreless Rating]], [[Star Ratings]]). There's no satisfaction score. [[GOAP]] doesn't read any of it.

A moment is a thought that happened: an event (a fall, a great run, a meal), or a condition (hungry, no lounge, bored) that held past a grace period of a quarter of a clock hour. Each kind has a class in `ai.Effects`:

- **Dealbreaker** (gave up, injured with no one coming, caught in an avalanche): the day is 1★. So is leaving without a run.
- **Letdown** (hungry, thirsty, nowhere to rest or warm up, priced out, bored or nothing their kind of skiing, every line too long, a minor injury): at most 2★.
- **Annoyance** (falls, tree hits, long lines, crowded, too hard or too easy runs, overpriced, shabby, packed, a slow patrol, renting in town): three make a letdown.
- **Highlight** (a great run, first tracks, a nice place, great après, patrol arriving fast): 3★ needs one. Without, the day was "nothing special", 2★.
- **Neutral**: counted and shown, no effect.

The level is 1–3; the quality of the services the guest used (every building visit and lift ride, `Guest.UseService`) adds up to 1.2 stars, capped at 5. When the car leaves the map the review (`world.Review`: stars, the moment that set them, the count) goes to the day's history, and the day's average stars becomes the resort rating at midnight ([[Demand]] takes it on 0–1). The guest popup shows the stars so far and the review line; the day's recap names the moment behind most reviews below 3★.

Each departure also records one reason (`DepartReason`). The Reviews tab in the charts window shows the day's reviews in aggregate: the average stars, the count at each level, what set each review, and the guests who went home because they couldn't rent skis, who leave no review and count as lost business.

Older docs call this Satisfaction or Fun.

Built by [[Satisfaction Rework]]; [[Snow Tastes]] adds per-guest snow tastes; [[Scoreless Rating]] replaced the ledger with moments.

Spec: [[Guests Spec]], Satisfaction, Rating, and Thoughts (written for the old score).

## Log

- 2026-10-01: Drift, the thought catalogue, and the rating average are in.
- 2026-10-07: Corrected: there is no first-ride bonus. Rework planned in [[Satisfaction Rework]].
- 2026-10-07: One effects table, condition thoughts, and mood in every activity with needs pulling it down ([[Satisfaction Rework]] steps 1–3).
- 2026-10-07: Departure reasons, the daily-average rating, service and patrol events, and run verdicts from a per-run summary ([[Satisfaction Rework]] steps 4–7).
- 2026-10-07: A per-guest baseline that experiences move ([[Mood Baseline]]); mood is now saved for guests on the mountain.
- 2026-10-07: Snow underfoot against tastes replaces the grooming and glade pulls ([[Snow Tastes]] step 2).
- 2026-10-07: A ledger: events add once, conditions cost by the clock hour, nothing drifts back; recorded when the car leaves the map, with each condition still on costing once more. Replaces the baseline and drift ([[Mood Baseline]]).
- 2026-10-07: Boredom ("skied this place to death", "nothing here is my kind of skiing") costs by the clock hour and sends the guest home; crowded runs cost as much as a guest dislikes crowds ([[Snow Tastes]] step 5).
- 2026-10-07: Service visits score relief by urgency, quality, value for money, crowding, and the wait at the door; "everything here costs too much" is a condition ([[Service Improvements]] step 2).
- 2026-10-07: Rentals, après (scores more after a good day), and the cold ([[Service Improvements]] step 3).
- 2026-10-08: Calibrated on the user's Boreal Goals Test save (three days, demand on): rating 0.46–0.50 → 0.50–0.60, mean final score 0.478 → 0.567. Two rule fixes: a run suits a guest when a quarter of its trail time is at their level or harder (`runLevelShare`), not most of it, so a short black that runs out on a long green isn't "too easy" (intermediates −0.134 → −0.010 a guest); and hunger and thirst press from 40% left (`mealFrom`), early enough to reach the food court before the condition starts at 15% (thirst −0.048 → −0.022). Per guest now: great runs +0.156, drinks +0.039, meals +0.020; falls −0.035, too easy −0.033 (all advanced, −0.28 each), thirst −0.022. By tier: beginners 0.62, intermediates 0.57, advanced 0.25. The map has no rental shop or lounge (cold, warming up, and renting in town cost about −0.02 together).
- 2026-10-09: The ledger replaced by moments and star reviews ([[Scoreless Rating]]): classes in `ai.Effects`, a grace period on conditions, quality stars from services and lift rides, the rating in stars.
- 2026-10-09: Renamed from Satisfaction.

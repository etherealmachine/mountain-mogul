---
title: Satisfaction
kind: stat
status: shipped
---

# Satisfaction

The session score, from 0 to 1. Starts at 0.6. [[GOAP]] does not read it. When the guest leaves, this number is what moves the resort rating in [[Demand]].

It drifts toward the terrain under their skis (trees, [[Grooming]], how that matches their traits), only while skiing, and jumps on events: a fall, tree hit, or injury in [[Skiing]] or an [[Avalanche]], being left by [[Ski Patrol]], a long line in the [[Lifts]], or no lodge to rest in. Each jump's size is hard-coded where it happens. Low [[Hunger]] or [[Thirst]] shows up as a thought and does not itself change the score. Running out of hunger, thirst, or [[Energy]] and going home is a clean exit. An exit from a broken [[Patience]] is not.

No [[Amenities]] write this stat directly. They change it by keeping the guest on the mountain, and by whether the resort had a lodge or a ticket window when the guest needed one.

Older docs and the F4 debug panel call this Fun. In the sim the field is `Satisfaction`.

Being rebuilt by [[Satisfaction Rework]]: one effects table for every event, thoughts as read-only reports, mood in every activity with needs pulling it down, departure reasons, and a daily-average rating.

Spec: [[Guests Spec]], the thoughts table (out of date: it lists a first-ride bonus and an off-piste thought that don't exist).

## Log

- 2026-10-01: Drift, the thought catalogue, and the rating average are in.
- 2026-10-07: Corrected: there is no first-ride bonus. Rework planned in [[Satisfaction Rework]].

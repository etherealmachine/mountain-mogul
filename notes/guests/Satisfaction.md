---
title: Satisfaction
kind: stat
status: shipped
---

# Satisfaction

The session score, from 0 to 1. Starts at 0.6. [[GOAP]] does not read it. When the guest leaves, this number is what moves the resort rating in [[Demand]].

It drifts toward the terrain under their skis (trees, [[Grooming]], how that matches their traits) and jumps on events: a fall or injury in [[Skiing]] or an [[Avalanche]], a first ride on one of the [[Lifts]], a long line. Low [[Hunger]] or [[Thirst]] shows up as a thought and does not itself change the score. Running out of hunger, thirst, or [[Energy]] and going home is a clean exit. An exit from a broken [[Patience]] is not.

No [[Amenities]] write this stat directly. They change it by keeping the guest on the mountain, and by whether the resort had a lodge or a ticket window when the guest needed one.

Older docs and the F4 debug panel call this Fun. In the sim the field is `Satisfaction`.

Spec: [[Guests Spec]], the thoughts table.

## Log

- 2026-10-01: Drift, the thought catalogue, and the rating average are in.

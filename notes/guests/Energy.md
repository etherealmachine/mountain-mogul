---
title: Energy
kind: stat
status: shipped
---

# Energy

How much skiing a guest has left in their legs, from 1 down to 0. [[GOAP]] reads it. Starts full. Drains during [[Skiing]], much faster on terrain above their skill and faster on snow the guest dislikes ([[Snow Tastes]]), and a fall takes a chunk off at once. About two hours of easy skiing from full to empty.

[[GOAP]] looks for a rest stop when energy (or [[Patience]]) falls below 0.15. A [[Lounge]] or a [[Food Court]] restores it to full. Below 0.05, `GoHome` wins.

Running out of energy is a normal end to the day. It does not dock [[Satisfaction]].

Older docs call this fatigue. In the sim it is this stat.

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, falls, and lodge rest are in.
- 2026-10-07: Snow a guest dislikes tires them faster.

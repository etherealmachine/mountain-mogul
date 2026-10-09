---
title: Patience
kind: stat
status: shipped
---

# Patience

How much waiting a guest will tolerate, from 1 down to 0. [[GOAP]] reads it. Starts full. Drains while standing in a line for one of the [[Lifts]] (not before the lifts open) and while walking without skis. Skiing and riding put a little back. A [[Lounge]] or a [[Food Court]] fills it in one rest. Closing time doesn't touch it, so the walk out still costs patience.

Below 0.15 the guest is sick of waiting: one thought, and a pull on [[Moments]] until patience recovers past 0.25. [[GOAP]] looks for that rest when patience (or [[Energy]]) falls below 0.15. Below 0.05, `GoHome` wins. A guest won't join a line longer than about half a clock hour of expected wait (about 56 people at 8 seconds each); one who finds every lift line they'd use over that cap thinks "every lift line is way too long" and leaves, a departure for lines. That exit does dock [[Moments]], via the long-line thought.

Not built yet: **traffic**. Patience should drain while a guest's car waits or crawls in traffic, arriving and leaving ([[Transit]]), so jams count. Today a guest's mood is recorded at the parking lot (`ActDepart`), before they wait in the car for their carload and before the drive out, and nothing drains patience or moves mood in a car. Counting traffic means keeping mood and the need conditions running in the car, perhaps a "stuck in traffic" condition, and recording the departure when the car leaves the map.

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Queue drain, lodge rest, and the full-queue bail are in.
- 2026-10-07: Low patience is now a condition that pulls satisfaction down ([[Satisfaction Rework]]).
- 2026-10-07: Closing no longer zeroes patience; noted traffic as a planned drain.
- 2026-10-07: Patience rates and the line cap are in clock units, so they kept their meaning when a clock hour became 900 sim seconds.

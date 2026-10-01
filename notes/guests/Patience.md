---
title: Patience
kind: stat
status: shipped
---

# Patience

How much waiting a guest will tolerate, from 1 down to 0. [[GOAP]] reads it. Starts full. Drains only while standing in a line for one of the [[Lifts]]. Skiing and riding put a little back. A [[Lounge]] or a [[Food Court]] fills it in one rest.

[[GOAP]] looks for that rest when patience (or [[Energy]]) falls below 0.15. Below 0.05, `GoHome` wins. A guest who finds every reachable lift line over its cap loses the rest of their patience at once and leaves unhappy. That exit does dock [[Satisfaction]], via the long-line thought.

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Queue drain, lodge rest, and the full-queue bail are in.

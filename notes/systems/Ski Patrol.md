---
title: Ski Patrol
kind: system
status: shipped
---

# Ski Patrol

Each patrol hut spawns one patroller on a snowmobile. An idle patroller claims the nearest injured guest, drives there, spends a few seconds loading them, and drives them to the parking lot, where the rescue goes into the [[Event Feed]]. The patroller then returns to the hut.

Injuries come from falls in [[Skiing]] and from being caught in an [[Avalanche]]. An injured guest waits where they fell, so the distance from a hut to the steep terrain is the response time. The injury has already cost the guest [[Satisfaction]] by the time patrol arrives.

Code: `internal/sim/patrol.go`, `internal/world/patroller.go`.

## Log

- 2026-10-01: Huts, one patroller each, the rescue cycle, and the event entry are in.

---
title: Ski Patrol
kind: system
status: shipped
---

# Ski Patrol

Patrol is a service in any building ([[Building Services]]): each patrol tile bases one patroller on a snowmobile, waiting outside the patrol door. An idle patroller claims the nearest injured guest, drives there, spends a few seconds loading them, and drives them to the nearest patrol room (first aid), where the rescue goes into the [[Event Feed]] and the guest heads home to their car. The patroller then returns to its door.

Injuries come from falls in [[Skiing]] and from being caught in an [[Avalanche]]. An injured guest waits where they fell, so the distance from a hut to the steep terrain is the response time. The injury has already cost the guest [[Satisfaction]] by the time patrol arrives.

Code: `internal/sim/patrol.go`, `internal/world/patroller.go`.

## Log

- 2026-10-01: Huts, one patroller each, the rescue cycle, and the event entry are in.
- 2026-10-06: Planned: patrol becomes a service in any building instead of a hut ([[Building Services]]).
- 2026-10-06: The patrol hut is removed from the build tools and saves; patrol returns as a building service ([[Building Services]]).
- 2026-10-06: Patrol is a building service: one patroller per patrol tile, patients to first aid ([[Building Services]]).

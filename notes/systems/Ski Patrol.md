---
title: Ski Patrol
kind: system
status: shipped
---

# Ski Patrol

Patrol is a service in any building ([[Building Services]]): each patrol tile bases one patroller. In the morning patrollers fetch snowmobiles from a garage and park them on the snow by the patrol room; an injury sends one out by the fastest way (the snowmobile, a lift and skis, or hiking) to the guest and back to first aid, where the rescue goes into the [[Event Feed]] and the guest heads home to their car. Patrol sweeps until the last guest is off the mountain, then puts the snowmobiles away ([[Patrol Day]]).

Injuries come from falls in [[Skiing]] and from being caught in an [[Avalanche]]. An injured guest waits where they fell, so the distance from a hut to the steep terrain is the response time. The injury has already cost the guest [[Moments]] by the time patrol arrives.

The player tracks falls three ways: a pin over every guest who is down (red while they get up, magenta while they wait for patrol), the Falls overlay (a heat map of where guests fell today), and the patrol popup's "Falls today" (the count, how many were off any run or getting off a lift, and the three runs with the most). A fall counts against the run the guest was skiing, even off its edge, else the run underfoot. Code: `internal/scene/fall_overlay.go`, `world.History.FallsToday`.

Code: `internal/sim/patrol.go`, `internal/world/patroller.go`.

## Log

- 2026-10-01: Huts, one patroller each, the rescue cycle, and the event entry are in.
- 2026-10-06: Planned: patrol becomes a service in any building instead of a hut ([[Building Services]]).
- 2026-10-06: The patrol hut is removed from the build tools and saves; patrol returns as a building service ([[Building Services]]).
- 2026-10-06: Patrol is a building service: one patroller per patrol tile, patients to first aid ([[Building Services]]).
- 2026-10-06: Diagnosed why patrol never rescues anyone (the snowmobile waits on the bare pad and won't move); [[Patrol Day]] plans the fix and a patrol day.
- 2026-10-06: Patrollers fetch and park snowmobiles, answer from the patrol room, and sweep after close; rescues work again ([[Patrol Day]] step 3).
- 2026-10-06: Patrollers answer by snowmobile or by lift and skis, whichever is faster, and bring guests down by toboggan; injured guests wait ten minutes of movement ([[Patrol Day]] steps 4–5).
- 2026-10-06: Patrollers hike to any injured guest when nothing is faster, so every injury gets a responder; a guest with help on the way waits for it ([[Patrol Day]] step 6).
- 2026-10-06: The patient is drawn while being loaded and towed by toboggan, and walks to their car after first aid instead of vanishing ([[Patrol Day]] step 6).
- 2026-10-08: Fall tracking: down-guest pins, the Falls heat-map overlay, and "Falls today" in the patrol popup.

---
title: Snowmobile Routes
kind: plan
status: idea
---

# Snowmobile Routes

Snowmobiles that find a way round things instead of driving straight at their target. Extends [[Ski Patrol]] and [[Pathfinding]]. Listed in [[Next Steps]].

## What's there today

- A patroller's snowmobile drives in a straight line from the garage to the call and back (`patrollerDrive`, `internal/sim/patrol.go`): 12 m/s on snow, 2 m/s over bare ground.
- Nothing is in its way: it goes through trees, buildings, lift towers and stations, guests, and over cliffs and open creeks, at full speed on any slope.
- Dispatch picks the snowmobile or the lifts by a straight-line time estimate (`sledTime`), so it can't tell when the straight line is impossible.

## The idea

1. **A route round obstacles.** Search the grid like walkers do (`Pathfinder`), but for a snowmobile:
   - trees, buildings, lift stations and towers are blocked
   - slopes too steep to climb or drop are blocked, depending on which way it's driving
   - bare ground is slow, and open creeks and lakes are blocked
   - groomed runs and cat tracks are preferred
   Cut the route to its corners, as walking routes are.
2. **Drive the route.** Follow it with a turning radius, slow down on steep ground and in corners, and give way to guests nearby, the way skiers avoid each other.
3. **Dispatch on the real route.** Use the route's length and speeds for the time estimate, so a call up a cliff band goes by lift instead.
4. **Later:** snowcats could use the same search for their transit between passes, which today runs straight over trees ([[Grooming]]).

## Open questions

- Should snowmobiles use the trail network ([[Trail Network]]) where they can, or search the open grid?
- Do guests react to a snowmobile coming through, by stepping or steering aside?

## Log

- 2026-10-10: Noted with the user: snowmobiles need pathfinding.

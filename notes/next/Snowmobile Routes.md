---
title: Snowmobile Routes
kind: plan
status: partial
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

## Built (2026-10-10)

- **Route** (`planSledRoute`, `internal/sim/sled_route.go`): A* over the whole map in seconds of driving. Blocked: buildings and two-tree cells (not `Walkable`), unfrozen lakes, cells within 3 m of a lift tower or station part, and steps steeper than a 0.65 climb (about 33°) or 0.75 drop (37°). Speed: 12 m/s on groomed snow, 70% of that ungroomed, 2 m/s bare, slower climbing; crossing a rope or the boundary adds 4 s. The start and end cells are allowed (a garage door, a guest in the trees). Cut to corners on lines that stay open, not too steep, and clear of ropes.
- **Driving** (`patrollerDrive`): replans when the target moves; turns at 1.6 rad/s, speeds up at 3 m/s² and brakes at 6, slows into turns and the bend at the next waypoint, slows to 2.5 m/s for a guest within 10 m ahead, and pulls up at the end. With no route it heads straight there, as before.
- **Dispatch** times the snowmobile by its route, so a call it can't reach goes by lift or on foot.
- **Measured** on Boreal with an injury every 15 minutes from 10:00 to 15:00: 26 snowmobile calls (33 before; the rest now go by lift), averaging 63 s for 436 m straight (33 s before, straight through everything), longest 122 s, none stuck. Ticks driving in trees or a building fell from 101 to 16, and within 2 m of a tower from 4 to 0.

## Left

- Creeks: the world has no creek cells to block yet ([[Creeks and Lakes]]).
- Snowcats' transits between passes still drive straight ([[Grooming]]).
- Guests don't step aside; the snowmobile just slows.
- Building the grid scans the whole map on every plan; fine for a few calls an hour, worth caching if snowcats use it.

## Open questions

- Should snowmobiles use the trail network ([[Trail Network]]) where they can, or search the open grid?
- Do guests react to a snowmobile coming through, by stepping or steering aside?

## Log

- 2026-10-10: Noted with the user: snowmobiles need pathfinding.
- 2026-10-10: Built the route search, driving it, and route-timed dispatch (see Built).

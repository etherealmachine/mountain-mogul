---
title: Rope Lines
kind: plan
status: idea
---

# Rope Lines

Ropes the player puts up to guide skiers: they can't cross them, so the player shapes where guests go without telling them where to ski. Fits the agentic-skier vision ([[Vision]]): the player changes the environment and guests decide for themselves. Extends [[Trails]] and [[Lifts]]. Listed in [[Next Steps]].

## What's there today

- The only ropes are the lift-line maze dividers on a double with lanes (`LiftQueueConfig`), and they're only drawn: guests stand in their places, and nothing stops a skier crossing.
- Skiers steer round trees, towers, stations and each other. Nothing else on the snow is an obstacle.
- Lines at service doors are laid out in code, with no ropes ([[Amenities]]).

## The idea

1. **A Rope tool.** Draw a rope as a line of nodes, like a footpath or a run, on bamboo poles. Ropes are cheap per metre, and can be moved or removed.
2. **Skiers avoid crossing ropes.** Steering and route choice try to stay on one side, but a rope isn't a wall: a skier who ends up crossing one just does, with no collision or fall. So a rope can:
   - funnel a run into a lift line
   - keep skiers off a lift's loading or unloading area
   - close off a cliff band, a creek, or a closed run
   - stop beginners getting onto a harder run where a cat track meets or crosses it (the "ropes and signs at trail splits" item in [[Next Steps]])
   - keep skiers apart where two runs merge
   Walkers avoid crossing ropes the same way ([[Pathfinding]]): crossing costs extra rather than being blocked.
5. **The ski area boundary is a rope.** The boundary the player owns ([[Parcels]], [[Land and Boundaries]]) is drawn and treated as a rope line, under the same rules: skiers avoid crossing it, but can.
3. **Mazes built by hand.** Ropes at a lift base or a door shape the line: guests stand along the roped lanes instead of a layout from code. The existing lift-line lanes could become a preset the tool places.
4. **Signs later.** "Slow", "Experts only" and "Closed" signs on a rope change who passes and how fast, instead of blocking. Slow zones and patrol enforcing them build on this ([[Ski Patrol]]).

## Open questions

- Do ropes need gaps, so the player can leave a gate?

## Log

- 2026-10-10: Noted with the user: rope lines to help guide skiers. Folds in "Ropes and signs at trail splits" (2026-10-08).
- 2026-10-10: The user: ropes are cheap; skiers try not to cross them, but crossing is fine and nothing collides; the ski area boundary is a rope with the same rules.

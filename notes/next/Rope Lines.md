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

1. **A Rope tool.** Draw a rope as a line of nodes, like a footpath or a run, on bamboo poles. It costs a little per metre and can be moved or removed.
2. **Skiers treat ropes as walls.** Steering avoids them like a line of trees, so a rope can:
   - funnel a run into a lift line
   - keep skiers off a lift's loading or unloading area
   - close off a cliff band, a creek, or a closed run
   - stop beginners getting onto a harder run where a cat track meets or crosses it (the "ropes and signs at trail splits" item in [[Next Steps]])
   - keep skiers apart where two runs merge
   Walkers treat ropes as blocked cells too ([[Pathfinding]]).
3. **Mazes built by hand.** Ropes at a lift base or a door shape the line: guests stand along the roped lanes instead of a layout from code. The existing lift-line lanes could become a preset the tool places.
4. **Signs later.** "Slow", "Experts only" and "Closed" signs on a rope change who passes and how fast, instead of blocking. Slow zones and patrol enforcing them build on this ([[Ski Patrol]]).

## Open questions

- Should a rope be a hard wall, or something most guests respect and a few duck under (and get a patrol warning)?
- Do ropes need gaps, so the player can leave a gate?

## Log

- 2026-10-10: Noted with the user: rope lines to help guide skiers. Folds in "Ropes and signs at trail splits" (2026-10-08).

---
title: Rope Lines
kind: plan
status: partial
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

## Built (2026-10-10)

- **Rope tool** in the Transport menu: click pole by pole, click the last pole again or press Enter to finish; right-click takes back a pole, Esc drops the rope. Poles snap to other ropes' poles. Editing a rope: drag a pole, right-click one to take it out, Shift-click to add one. Remove takes down a whole rope. $4 a metre, owned land only. Poles every 3 m, drawn like the lift-line ropes (`internal/render/rope_mesh.go`, shared with them). Saved.
- **Skiers** (`sampleTactical`): each direction they consider loses `ropePenalty` (2) per sample beyond where it would cross a rope, as leaving the ski area already cost `boundaryPenalty` (8).
- **Routes** (`planSkiRoute`, the walkers' `Pathfinder`): crossing a rope or the ski area boundary costs 12 cells of going (`ropeStepCost`), instead of the boundary being a wall; corners are only cut on lines that keep 2 m clear of ropes (`ropeClear`) and on one side of the boundary. A run's trail route that crosses a rope gives way to a searched one. A waypoint by a rope's end counts as reached only once the way on is clear of the rope.
- **Measured** with a 60 m rope across the slope 40 m above Boreal's Lift4: over a day, guests not on a chair crossed the line 628 times without the rope and 259 with it, more than half of those within 5 m of an end; passes round the ends went from 179 to 482. Crossings through the middle 30 m fell from 238 to 37.

## Left

- Ropes in the scenario editor.
- Hand-built mazes: guests standing along roped lanes, and lift-line lanes as a rope preset.
- Signs, slow zones and gates.
- The ski area boundary is still drawn by its own fence code, and on 5 m cell edges.
- Snowmobiles and snowcats ignore ropes.

## Open questions

- Do ropes need gaps, so the player can leave a gate?

## Log

- 2026-10-10: Noted with the user: rope lines to help guide skiers. Folds in "Ropes and signs at trail splits" (2026-10-08).
- 2026-10-10: The user: ropes are cheap; skiers try not to cross them, but crossing is fine and nothing collides; the ski area boundary is a rope with the same rules.
- 2026-10-10: Built the Rope tool, skier and route avoidance, and the soft boundary (see Built).

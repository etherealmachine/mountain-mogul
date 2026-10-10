---
title: Ski Racks
kind: plan
status: idea
---

# Ski Racks

Guests stop to take their skis off and put them on, leave them outside in a rack while they're in a building, and pick them up when they come out. Extends [[Amenities]] and [[Skiing]]. Listed in [[Next Steps]].

## What's there today

- Changing in or out of skis is a 1 sim-second pause (`maybeStartSkiTransition`, `tickSkiTransition` in `internal/sim/skiing.go`). That's about a quarter of a real second at normal speed, with no animation: the figure just swaps skis on the feet for skis on the shoulder.
- Guests carry their skis into every building and out again. Nothing is left outside, and the base area has no racks.
- Rental guests get their gear inside the [[Rental Shop]] and come out carrying it.

## The idea

1. **A real pause.** Taking skis off or putting them on takes time: about 30 sim seconds for most guests, longer for beginners and rental gear, shorter for experts. The guest stops and bends to the bindings, with an animation for each. Groups wait for each other ([[Groups]]).
2. **Skis stay outside.** Before going into a building, a guest leaves their skis in the nearest rack with room, then walks in. When they come out they walk back to the rack, take their skis, carry them to the snow, and put them on. Pass holders and guests with their own gear do this every visit. Rental guests do it from their second visit on.
3. **Racks.** A rack is something the player places, probably a small object or a building service tile at the edge of the snow, with a capacity (about 10 pairs per rack). Skis left in it are drawn there, so a full base area looks busy.
4. **No rack nearby.** Guests stick their skis in the snow or lean them on the wall by the door. That's clutter, and it gets a thought ("nowhere to leave my skis"), and it's what makes the player build racks. Lost or stolen skis could come later.
5. **Where racks go:** where footpaths meet the snow (already in [[Next Steps]]), by lodge doors, and at the bottom of lifts for guests walking to lunch.

## Open questions

- Is a rack a placed object (like a snow gun) or a service tile on a building?
- Does the walk to and from the rack count toward the service's wait, or toward walking patience?

## Log

- 2026-10-10: Noted with the user: guests should pause to take skis off and put them on, and leave them outside, which needs racks. Folds in the earlier "Ski racks" line and the rack part of the [[Rental Shop]] item.

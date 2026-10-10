---
title: Ski Racks
kind: plan
status: partial
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

## Decided (the user, 2026-10-10)

- A rack is placed in the ground and sits on top of the snow.
- Taking skis off or putting them on takes about 4 real seconds at normal speed.
- Rental guests use racks like anyone else while they have gear, until they return it.
- With no rack, skis go in the snow, with no complaint for now; it should affect routing later.

## Built (2026-10-10)

- **Ski Rack** in the Transport menu: $1,500, an A-frame for 10 pairs (`models-src/ski_rack.scad`), rotatable, removed with Remove. Its popup shows how many pairs it holds.
- **Changing skis** takes `skiChangeTime`: 15 sim seconds (about 4 real seconds at normal), 1.3× for a beginner down to 0.7× for an expert, bent down to the bindings with the skis at their feet.
- **Leaving skis** (`internal/sim/gear.go`): a guest carrying skis on the way to use something in a building, within 40 m of the door, walks to the rack nearest the door with a free slot, stands in front of it for 3 sim seconds, and leaves the pair leaning on the rail. With no rack in reach, they stick the pair in the snow 12 m short of the door. One who gets to the door still holding skis leaves them on the spot.
- **Picking them up**: when their plan next takes them anywhere but into a building, they walk back to the pair, stop 3 sim seconds, and carry it on. They never put skis on while their pair is somewhere else.
- **Drawn** where they stand: in a rack, leaning on the rail, or upright in the snow.
- On Boreal's busy lunch with one rack by each lodge door, the racks filled (17 pairs) and the rest went in the snow (119); a whole day ran with no stuck guests.

## Left

- Where skis are isn't saved: after a load everyone is carrying again.
- Skis in the snow should change routing (a cluttered door is slower to get to), per the user.
- Returning rental gear, so rental guests stop using racks.
- Guests walk straight to a rack and back, round towers but not trees or buildings.
- Groups don't wait for each other at the rack.

## Log

- 2026-10-10: Noted with the user: guests should pause to take skis off and put them on, and leave them outside, which needs racks. Folds in the earlier "Ski racks" line and the rack part of the [[Rental Shop]] item.
- 2026-10-10: Built the rack, the longer ski change, leaving skis in racks or the snow and picking them up (see Built).

---
title: Guest Types
kind: concept
status: partial
---

# Guest Types

A guest is anyone who comes to the resort, not only skiers. The code calls them guests on purpose, so snowboarders, children, non-skiers, and later staff can share one model. None of them behave differently yet. This card tracks what's in place and what would get in the way.

## In place

- **One guest record** in `internal/world/guest.go`, used both for the 10,000-person catchment and for guests on the mountain.
- **Discipline**: the equipment a guest rides, either Ski or Snowboard. It's saved per guest, and about 20% of the catchment rolls Snowboard.
- **Skill** and the traits derived from it (comfort speed, comfort slope, aggression, daily budget).
- **Tastes**: seven affinities (groomed, powder, moguls, trees, steep, ice, crowds) rolled around an archetype that's more or less common by skill: Cruiser, Powder Hound, Bump Skier, Glade Rat, Charger ([[Snow Tastes]]). The follow panel names the nearest archetype. Tastes steer their line, choose their lift, judge each run, and decide when they've had enough of a resort.

## Not wired up

Nothing reads Discipline outside the save and the catchment roll. Snowboarders ski with ski physics, draw with the skier mesh, and aren't labeled in the follow-guest panel. That's fine for now; the point is not to block them later.

## Things that would get in the way

None need fixing now. Each one is where a new type would hit a wall:

- **One goal set for everyone.** [[GOAP]] ranks the same global goal list for every guest. Non-skiers, children, and staff need different goals, so the goals a guest considers should come from their type.
- **Traits come mostly from skill.** Tastes are separate, but children, snowboarders, and non-skiers need traits from more than one number (skill plus discipline plus age).
- **"Children" isn't a discipline.** Age is its own axis: a child can ski or board. Families also arrive and move together, which suggests parties of guests.
- **Non-skiers would be turned away.** Arrival assumes every guest needs a lift ticket ([[Tickets]], [[Demand]]). A non-skier needs a third discipline value, "none", and their own goals.
- **Skiing words in sim state.** `SkisOn`, the ski-to-parking actions, the keep-skiing goal, `tickSkier`, and the skier mesh are all named for skiing. Renaming them to equipment-neutral names is mechanical and can wait until boarding behaves differently. Saves store these as numbers, so renaming doesn't break old saves.

## When snowboarders get behavior

- A snowboarding physics module next to `internal/sim/skiing.go`, chosen per guest by Discipline
- A board mesh ([[Model Pipeline]], [[Rendering]])
- What boarders like and dislike: terrain parks, flat runouts and traverses, surface lifts ([[Trails]], [[Lifts]])
- Skiers-only resorts like [[Mad River Glen]] and [[Alta]] ([[Scenario Goals and Rules]])

## Log

- 2026-10-02: Checked the code. Discipline (Ski or Snowboard) is saved and rolled at 20% Snowboard, but nothing uses it yet.
- 2026-10-07: Tastes replace the glade and groomer flags ([[Snow Tastes]] step 1).
- 2026-10-07: Tastes now drive steering, lift choice, run verdicts, and boredom ([[Snow Tastes]]).

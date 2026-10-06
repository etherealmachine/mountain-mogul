---
title: Lift Operations
kind: plan
status: planned
---

# Lift Operations

Lifts that stop, hold, and carry people down as well as up, and that can't be built through each other. Extends [[Lifts]]. Listed in [[Next Steps]].

## What's there today

- Lifts run whenever the resort is open, except for snow holds. Wind exists in [[Weather]] but doesn't stop anything.
- Every rider goes up. Nobody rides down.
- Placement only checks that the bottom and top stations sit on owned land ([[Parcels]]). A new lift can overlap another lift's station, sit on a building, or cross another lift's cable.

## Steps

1. **No overlapping lifts.** The ghost turns red and placement is refused when a station overlaps another station, a building, a parking lot, or a road, or when the line crosses another lift's line or towers. Real resorts do run one lift over another, so a crossing with enough height clearance could be allowed later. The toast says what's in the way. Buildings and lots already refuse overlaps through the cells under them (`Ground`, `PaintedCellFree`; see [[Rotated Buildings]] and [[Transit]]): lift stations should join that check both ways, so a building can't be painted onto a station either.
2. **Breakdowns.** Lifts wear with every ride and break down at random, more often when worn or old. A stopped lift holds its riders on the line, and guests on it and in its queue lose [[Satisfaction]]. Short stops restart; long ones need an evacuation by [[Ski Patrol]] (patrollers from the patrol service, [[Building Services]]), which costs time, money, and rating. A maintenance contract or a mechanic on staff cuts wear and repair time ([[Finance]], Staff in [[Next Steps]]). Each breakdown goes in the [[Event Feed]].
3. **Wind holds.** Each lift type has a wind limit: chairs hold first, especially lifts on exposed ridges and tops, and gondolas and trams run longer. A held lift unloads what's on it and stops loading; guests in line wait a while, then pick another lift or leave. The lift reopens when the wind drops. Needs wind that varies by day and by hour (already in [[Next Steps]]).
4. **Downloading.** Guests can ride a lift down:
   - beginners who reach a top with nothing at their level
   - anyone at the end of the day, or when they're tired or hurt
   - everyone, when the lower mountain has no snow (south faces in spring, as at [[Kirkwood]]; see [[Terrain Realism]])
   - gondolas and trams download freely; chairs only if the lift allows it, which the player can toggle per lift
   Riders going down fill chairs that would otherwise return empty, so they don't cut uphill capacity, but they need their own line at the top station.

## Open questions

- Should breakdowns be pure chance weighted by wear, or tied to weather (cold starts, icing)?
- Does a crossing with clearance need a costlier tower, or is it free if the cables are far enough apart?

## Log

- 2026-10-05: Planned from the user's notes: lift breakdowns, wind holds, downloading, and overlapping lifts.
- 2026-10-06: Linked overlap refusal to the building and lot overlap checks, and evacuations to the patrol service.

---
title: Lift Operations
kind: plan
status: partial
---

# Lift Operations

Lifts that stop, hold, and carry people down as well as up, and that can't be built through each other. Extends [[Lifts]]. Listed in [[Next Steps]].

## What's there today

- Lifts run whenever the resort is open, except for snow holds. Wind exists in [[Weather]] but doesn't stop anything.
- Every rider goes up. Nobody rides down.
- Placement only checks that the bottom and top stations sit on owned land ([[Parcels]]). A new lift can overlap another lift's station, sit on a building, or cross another lift's cable.

## Decisions

From the user, 2026-10-14:

- **Wind first**, with wind holds; small. Daily wind that varies by hour comes with it.
- **Testing.** A lift can run empty with nobody boarding. A wind hold ends with a test run before it reopens, and guests may gather in its line while it tests. The player can also put a lift on a test run by hand, just to watch it run, even with the resort closed.

## Steps

1. **No overlapping lifts.** The ghost turns red and placement is refused when a station overlaps another station, a building, a parking lot, or a road, or when the line crosses another lift's line or towers. Real resorts do run one lift over another, so a crossing with enough height clearance could be allowed later. The toast says what's in the way. Buildings and lots already refuse overlaps through the cells under them (`Ground`, `PaintedCellFree`; see [[Rotated Buildings]] and [[Transit]]): lift stations should join that check both ways, so a building can't be painted onto a station either.
2. **Breakdowns.** Lifts wear with every ride and break down at random, more often when worn or old. A stopped lift holds its riders on the line, and guests on it and in its queue lose [[Moments]]. Short stops restart; long ones need an evacuation by [[Ski Patrol]] (patrollers from the patrol service, [[Building Services]]), which costs time, money, and rating. A maintenance contract or a mechanic on staff cuts wear and repair time ([[Finance]], Staff in [[Next Steps]]). Each breakdown goes in the [[Event Feed]].
3. Done: **Wind holds and testing.** (Built before steps 1 and 2.) Each lift type has a wind limit: chairs hold first, especially lifts on exposed ridges and tops, and gondolas and trams run longer, the funitel (built steady in wind, two haul ropes a way) longest of all. A held lift unloads what's on it and stops loading; guests in line wait a while, then pick another lift or leave. When the wind has stayed under the limit a while, the lift runs empty on a test run, guests free to queue for it, and then opens. A Test run switch in the lift popup runs it empty by hand, the resort open or not. Needs wind that varies by day and by hour, built with it.
4. **Downloading.** Guests can ride a lift down:
   - beginners who reach a top with nothing at their level
   - anyone at the end of the day, or when they're tired or hurt
   - everyone, when the lower mountain has no snow (south faces in spring, as at [[Kirkwood]]; see [[Terrain Realism]])
   - gondolas and trams download freely; chairs only if the lift allows it, which the player can toggle per lift
   Riders going down fill chairs that would otherwise return empty, so they don't cut uphill capacity, but they need their own line at the top station.

## Built: wind, holds and test runs

- **Wind.** Each day has a mean wind at the base and a direction (`DayWeather.WindMS`, `WindDeg`): by weather (clear 3.5 m/s, overcast 5, light snow 6, heavy snow 8.5, rain 6) times a log-normal spread (sigma 0.45), the direction about the climate's prevailing one. Through the day it's 15% stronger at 15:00 than at 03:00 and gusts hour to hour by 0.8–1.3 (`sim/wind.go`). At a lift's top it's 60% stronger a kilometre above the base, and up to half as strong again on a top standing 60 m over the ground 150 m around. Dry days with 7.5 m/s or more at the base turn the surface to wind slab (it was a random roll by temperature).
- **Holds.** Limits at the top (`LiftType.WindLimit`): fixed chairs 16 m/s, detachable chairs 18, the gondola 21, the T-bar 24, the tram 24, the funitel 28, the 3S 30; the carpet and rope tow never hold. Past it the lift holds like a snow hold: riders come up, its line goes, nobody boards (`Lift.WindHold`; `OnHold` is either hold).
- **Test runs.** Once the wind's stayed under 85% of the limit for half an hour, the lift runs empty for 20 minutes, guests free to queue, then opens (`Lift.TestUntil`). The lift popup shows the wind at the top and has a Test run switch that runs it empty by hand, open or closed (`Lift.Testing`, saved). Holds and test runs go in the feed.
- **On the Boreal Goals Test:** over 14 game days, one gusty day (9.5 m/s at the base) held Lift1 and Lift3 for about an hour. A forced 12 m/s day held the upper two all day; Lift4 ran the morning, held at 12:37, tested at 14:10 and reopened, and held again at 15:29.

## Open questions

- Should breakdowns be pure chance weighted by wear, or tied to weather (cold starts, icing)?
- Does a crossing with clearance need a costlier tower, or is it free if the cables are far enough apart?

## Log

- 2026-10-05: Planned from the user's notes: lift breakdowns, wind holds, downloading, and overlapping lifts.
- 2026-10-06: Linked overlap refusal to the building and lot overlap checks, and evacuations to the patrol service.
- 2026-10-14: Wind holds and testing ranked first with the user; daily wind folded in.
- 2026-10-14: Wind, wind holds and test runs built (step 3).

---
title: Lift Lines
kind: plan
status: planned
---

# Lift Lines

How guests wait for a lift. With no line attendant, a line is a loose crowd. With one, the attendant puts up a rope maze sized to the line, with a lane for groups, a singles lane and a patrol lane, and fills each chair. From the [[Boreal Playtest]] (item 9: "line out the parking lot looks bad - attendant can auto-manage line, need merging lines with multiple lanes and groups + single rider and ski patrol"). Extends [[Lifts]]; ropes from [[Rope Lines]]; groups from [[Groups]]. Listed in [[Next Steps]].

## What's there today

- Two line models. Every lift but a double has one single-file line (`Lift.Queue`) running straight out from the base. A double can instead have lanes the player sets up in the lift popup (`LiftQueueConfig`: 0–5 pair lanes on each side, single-rider lanes, rope depth), drawn with ropes that don't stop anyone.
- The single-file line folds into switchback lanes before lots and blocked cells (`UpdateQueueFolds`, `internal/world/lift_queue_fold.go`). This is a stopgap from the playtest that holds no matter who is staffing the lift. Arrivals walk round the outside of the line to its back (`QueueApproachXZ`, sticky `Guest.LineWay`).
- Boarding (`internal/sim/boarding.go`). With an attendant ($750 a day), every seat is filled: groups get split and singles called up. Without one, the next group joins a chair only if they all fit, and even then only 90% of the time. Unattended quads on Boreal leave about 75% full with a line behind them.
- Patrollers don't queue. They walk to the base and ride a lift that isn't drawn (`PatrollerToLift` → `PatrollerRiding`, `internal/sim/patrol.go`).
- Guests won't join a line that's too long by the clock, and lines drain [[Patience]].

## Decided (2026-10-12, with the user)

- **No attendant: a loose crowd.** Guests bunch in a rough wedge that fans out from the load. They board mostly in the order they arrived, but some slip ahead, groups clump together, and the crowd spreads sideways instead of growing longer. A guest who gets passed is annoyed.
- **Attendant: an automatic maze.** Hiring an attendant puts up a rope maze sized to the line. Its switchbacks grow and shrink with the line and keep out of lots, buildings and the station. The player doesn't lay it out. The doubles' lane settings (`LiftQueueConfig`) go away. Player-built rope mazes can come later ([[Rope Lines]] "Left").
- **Lanes in a maze:** groups (the maze itself), a singles lane, and a patrol/staff lane. A ski school lane waits for lessons.
- **Merging:** the attendant fills each chair. The front group in the maze boards first, singles fill the empty seats, and lanes take turns so none waits forever. This builds on what today's attendant already does.

## How it works (to build)

- **One line, two layouts.** Each lift keeps one list of guests in boarding order, plus a layout that places each guest: crowd or maze, depending on whether the lift has an attendant. The double's `Lines` and `LiftQueueConfig` are removed. Old saves break, which is fine.
- **The crowd.** Places fill a wedge in front of the load, in rows about 1.2 m deep that widen by one place per row up to about 6 across. Rows step around lots, buildings and blocked cells (the wedge leans sideways rather than running into them).
  - Arrivals take the nearest free place on the crowd's edge, beside or behind it, so they don't walk through anyone.
  - Boarding order is roughly by distance to the load. Each time a place opens near the front, a guest beside it can slip in ahead of someone who arrived earlier. Passed guests may think "people keep cutting in" (an annoyance, weighted lightly). Groups take neighbouring places.
- **The maze.** Switchback lanes 1.5 m apart, laid out in front of the load on the side away from the operator's hut (reusing the fold code's checks for blocked cells and lots).
  - The maze has room for the line plus a margin. It re-lays every clock hour, and sooner when the line outgrows it. Lanes are only added or removed at the back, so nobody already standing in it has to move.
  - The ropes are drawn with the rope mesh. Skiers and walkers treat them as ropes, so they go round to the entrance.
  - Order is strict: arrivals join at the entrance and walk the lanes to their place, and nobody slips ahead.
  - When there's no room left, the overflow waits as a crowd at the entrance.
- **The singles lane** is a straight lane beside the maze, running to the load.
  - A guest on their own takes it when it promises the shorter wait.
  - A group whose members don't mind splitting up may send its members there separately, as long as they agree to meet at the top.
  - The attendant fills empty seats on each chair from it.
- **The patrol lane** is a short open lane on the other side. Patrollers walk it to the load and board a real chair (their ride stops being hidden). Later, instructors and other staff use it too.
- **The lift popup** loses the lane settings. With an attendant, it shows how many are in the maze and in the singles lane and the wait for each. Without one, it shows the crowd and how full chairs leave (as now).

## Steps

1. **One line model.** Merge `Queue` and the doubles' `Lines` into one boarding-order list with a layout that gives each guest a place. Remove `LiftQueueConfig`, its popup rows and its save fields. Boarding (`loadChair`) reads the one list. Keep today's single-file layout, folds included, so nothing looks different yet. Check it with `go build` and `go vet`, and run the Goals Test for two headless days: ratings, waits and fill should match today.
2. **The crowd.** Lifts with no attendant use the wedge layout, the rule for taking an edge place, slipping ahead, and the cut-in annoyance. The fold stopgap goes away. Re-measure on the Goals Test against the playtest numbers: no place in a lot, how often arrivals pass through someone (615 now), how often guests slip ahead, how full chairs leave, and the line complaints.
3. **The maze.** Lifts with an attendant get the automatic switchback maze: ropes, entrance, strict order, re-laying from the back, and the overflow crowd. Ropes steer arrivals round to the entrance. Re-measure with attendants on Boreal's lifts: arrivals cutting through ropes, places in lots, and wait.
4. **The singles lane.** The lane beside the maze, the choice to take it (singles, and groups willing to split who regroup at the top, [[Groups]]), and the attendant filling seats from it. Measure: chair fill and the wait in each lane.
5. **The patrol lane.** Patrollers walk the lane and ride a real chair. Check that a patroller heading to an injury doesn't wait behind guests.
6. **Popup and readouts.** The lane counts and waits in the lift popup, and the line tooltip. Update [[Lifts]] and [[Rope Lines]].

## Open questions

- How wide and deep can a crowd get before it looks like a crowd at a door rather than a lift line? About 6 across is the starting guess.
- How often should guests slip ahead, and how much should being passed hurt? Start with a light annoyance and tune it once it's visible.
- Does a group decide to split into the singles lane as a group, or does each member decide? The default is the group decides, by the leader's patience.
- Should the attendant's maze cost anything beyond the attendant's wage? The default is no: the ropes come with the attendant.

## Log

- 2026-10-12: Planned with the user from playtest item 9: a loose crowd with no attendant; with one, an automatic rope maze with groups, singles and patrol lanes, and the attendant filling each chair. The doubles' lane settings go away.

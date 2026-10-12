---
title: Lift Lines
kind: plan
status: started
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

- **One line, two layouts.** Built in step 1: each lift keeps one list of guests in boarding order, plus a layout that places each guest: crowd or maze, depending on whether the lift has an attendant. The double's `Lines` and `LiftQueueConfig` are removed. Old saves break, which is fine.
- **The crowd** (built in step 2, `internal/world/lift_crowd.go`). Up to 300 places (`Lift.Crowd`, laid out by `UpdateLineLayouts` at the start and hourly, and when the attendant is hired or let go): the loading spot, then staggered rows 1.2 m apart with places 1 m apart, each jittered up to 0.25 m. A place's cost is metres along the line plus 2.4 a metre out to 3 m from the axis and 6 a metre beyond, so the crowd fans out about 6 across and goes wider only round something in its way. Places leave out cells that aren't walkable, lots, buildings, the station's legs (1 m) and its hut (0.5 m); at most 50 m along and 8 m out.
  - Arrivals head for the free place on the crowd's edge nearest them (`CrowdArrivalXZ`): no dearer than the dearest occupied place plus 1.5, and no cheaper than it less 2.4, so nobody takes a gap deep inside. Someone joining their group stands by them (up to 3 dearer).
  - The line (`Lift.Queue`) stays in the order guests came. After each chair loads, everyone shuffles up: each free place, front first, goes to whoever came first among those within 2.6 m behind it, except that 10% of the time the nearest of the others gets there first and goes ahead of them in the line (`ShuffleCrowd`). Passed twice in one wait, a guest thinks "people keep cutting in" (`ThoughtCutIn`, an annoyance).
  - Boarding takes the front of the line and the rest of their group in it from wherever they stand. Without an attendant, the next group shares the chair only if the rest of them standing right behind fits (as before).
  - Walkers leave their route for their place within 10 m of it, and the last cell of a route to a lift heads for the line itself (its centre could be in the station's way).
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

1. Done: **One line model.** Every lift has one line, `Lift.Queue`, in boarding order; where each guest stands is `QueueSlotXZ` (the layout). The doubles' lanes, `LiftQueueConfig`, their popup rows, rope meshes and save fields are gone; no shipped save used them. Goals Test over two days unchanged (2.82★ and 2.83★, falls 597 and 434). The plan was: merge `Queue` and the doubles' `Lines` into one boarding-order list with a layout that gives each guest a place. Remove `LiftQueueConfig`, its popup rows and its save fields. Boarding (`loadChair`) reads the one list. Keep today's single-file layout, folds included, so nothing looks different yet. Check it with `go build` and `go vet`, and run the Goals Test for two headless days: ratings, waits and fill should match today.
2. Done: **The crowd.** See How it works. On the Goals Test (no attendants), one day: nobody in a lot; arrival-seconds within 0.6 m of someone in line 3,654 → about 3,500 at Lift1, 6,249 → 605 at Lift4, 159 → 7 at Lift3 (Lift1's are the morning crowd of about 230 waiting for opening, walked through by arrivals from the front: routes don't know where the crowd stands); the furthest anyone stands from the base 44 → 39 m; chairs leave about as full; boardings 3–7% more. But over six seeds the rating is 2.73★ against 2.80★: the crowd puts more guests in line (6–8% more joins a day, arrivals reaching it 25% sooner), the mean wait is 10–30% longer, more run out of patience in line and go to rest, and the Goals Test's small lodge overflows ("it's packed in there" about 215 → 300–360 a day, "this place needs a lodge" 9 → 60–80). "People keep cutting in" about 19 a day. Not fully explained why joins rise beyond the quicker arrival. The single-file fold stays for lifts with an attendant until step 3. The plan was: lifts with no attendant use the wedge layout, the rule for taking an edge place, slipping ahead, and the cut-in annoyance. The fold stopgap goes away. Re-measure on the Goals Test against the playtest numbers: no place in a lot, how often arrivals pass through someone (615 now), how often guests slip ahead, how full chairs leave, and the line complaints.
3. **The maze.** Lifts with an attendant get the automatic switchback maze: ropes, entrance, strict order, re-laying from the back, and the overflow crowd. Ropes steer arrivals round to the entrance. Re-measure with attendants on Boreal's lifts: arrivals cutting through ropes, places in lots, and wait.
4. **The singles lane.** The lane beside the maze, the choice to take it (singles, and groups willing to split who regroup at the top, [[Groups]]), and the attendant filling seats from it. Measure: chair fill and the wait in each lane.
5. **The patrol lane.** Patrollers walk the lane and ride a real chair. Check that a patroller heading to an injury doesn't wait behind guests.
6. **Popup and readouts.** The lane counts and waits in the lift popup, and the line tooltip. Update [[Lifts]] and [[Rope Lines]].

## Open questions

- How wide and deep can a crowd get before it looks like a crowd at a door rather than a lift line? About 6 across is the starting guess.
- How often should guests slip ahead, and how much should being passed hurt? Built at 10% of shuffles, a light annoyance on the second pass; tune it once it's visible.
- The crowd lowers the Goals Test's rating by about 0.06★, through longer waits filling its small lodge. Accept it as the lodge's problem, or slow the crowd down?
- Does a group decide to split into the singles lane as a group, or does each member decide? The default is the group decides, by the leader's patience.
- Should the attendant's maze cost anything beyond the attendant's wage? The default is no: the ropes come with the attendant.

## Log

- 2026-10-12: Planned with the user from playtest item 9: a loose crowd with no attendant; with one, an automatic rope maze with groups, singles and patrol lanes, and the attendant filling each chair. The doubles' lane settings go away.
- 2026-10-12: Step 1 done: one line model; the doubles' lanes removed.
- 2026-10-12: Step 2 built: the crowd, with edge joining, shuffling up, slips and the cut-in annoyance. Rating on the Goals Test 2.80 → 2.73★ over six seeds (longer waits fill its lodge); to judge in the running game.

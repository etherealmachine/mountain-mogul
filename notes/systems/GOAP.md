---
title: GOAP
kind: system
status: partial
---

# GOAP

The guest's strategic layer, in `internal/ai/goap/`. Each guest has a snapshot of their own state, a handful of goals, and actions with preconditions, effects, and costs. At a replan the highest-weighted unsatisfied goal wins, and a search picks the cheapest chain of actions that satisfies it. The head action is what the skiing controller steers toward. New behavior is a goal plus an action, not a new state machine. Every guest weighs the same global goal list today; non-skiers, children, and staff will need their own sets (see [[Guest Types]]).

It replans when the plan is empty, when the head action finishes, or when that action's precondition breaks. A need that crosses its threshold mid-plan can preempt the next step, so a guest who gets hungry on a lap heads for food instead of finishing the lap. It does not poll on a timer.

Needs are planned as urgencies, 0..1: hunger, thirst, and rest (the lower of [[Energy]] and [[Patience]]). One goal shape, `FulfillNeed`, weighs each by its own curve (`needSpecs`), and one action, `UseService`, uses whatever a building offers (a seat, a meal, a drink), each offer declaring the needs it fulfils; the planner never sees the stats behind the urgencies, and the sim applies what an offer does when it finishes ([[Service Improvements]]). The skiing goals still read energy and patience directly. [[Moments]] is not a planning input; it is the score the resort rating reads when the guest leaves. Tastes are: a lift costs more to ride when the terrain off its top doesn't suit the guest ([[Snow Tastes]]), and a guest who's bored of everything they'd ride heads home ahead of more skiing. [[Tickets]] gate whether a plan may board a lift.

Where those needs get relieved is [[Amenities]]. Plans are rides on [[Lifts]] and runs along [[Trails]]; [[Skiing]] carries out each run.

The long spec is [[Guests Spec]]. It lags the code in places (the action table predates the bar and the food court); trust the package when they disagree, and update the spec when a behavior changes.

## Log

- 2026-10-01: Ski loop, rest, hunger, thirst, tickets, and need preemption are in. The planner is still growing one goal at a time.
- 2026-10-06: When a guest can't plan a lift ride, the thought says why: "the lifts are all closed", "there's nothing here I can ski" (no running lift serves their level), or, only when neither, "couldn't find where to buy a ticket" (`rideBlocker` in `planner.go`). Before, every such failure was blamed on the ticket window. Planner tests fixed: their lifts start stopped, their guests now hold a ticket, and the thirst test uses critical thirst, the only level that outweighs skiing at full patience.
- 2026-10-07: Guests ride lifts serving trails at or below their level, preferring their own level (a lift without one costs 240 s more). The planner no longer writes guest state: what blocked a goal comes back in `Plan.Blocked` and the sim turns it into conditions ([[Satisfaction Rework]]).
- 2026-10-07: At closing, `GoHome` weighs 2.0 (above every other goal) while `World.ClosedForDay` is set, instead of the sim zeroing everyone's patience; stats are left alone so the trip home still costs patience.
- 2026-10-07: Lift choice reads the guest's tastes against each lift's trail conditions (and its line as crowding); boredom puts `GoHome` just above skiing ([[Snow Tastes]]).
- 2026-10-07: Needs as urgencies: `FulfillNeed` replaces `RelieveHunger`, `RelieveThirst`, and `Rest`; `SkiToService` and `UseService` replace the lodge and bar descents and the rest, meal, and drink actions; one `AtService` anchor replaces `AtLodge` and `AtBar` ([[Service Improvements]] step 1). Behaves as before within seed noise.
- 2026-10-07: Rolled needs (rentals as a gate on `JoinQueue`, après, warmth) and `WalkToService` from the base area ([[Service Improvements]] step 3).

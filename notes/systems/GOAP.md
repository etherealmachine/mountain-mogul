---
title: GOAP
kind: system
status: partial
---

# GOAP

The guest's strategic layer, in `internal/ai/goap/`. Each guest has a snapshot of their own state, a handful of goals, and actions with preconditions, effects, and costs. At a replan the highest-weighted unsatisfied goal wins, and a search picks the cheapest chain of actions that satisfies it. The head action is what the skiing controller steers toward. New behavior is a goal plus an action, not a new state machine. Every guest weighs the same global goal list today; non-skiers, children, and staff will need their own sets (see [[Guest Types]]).

It replans when the plan is empty, when the head action finishes, or when that action's precondition breaks. A need that crosses its threshold mid-plan can preempt the next step, so a guest who gets hungry on a lap heads for food instead of finishing the lap. It does not poll on a timer.

The needs it weighs are [[Hunger]], [[Thirst]], [[Energy]], and [[Patience]]. [[Satisfaction]] is not a planning input; it is the score the resort rating reads when the guest leaves. [[Tickets]] gate whether a plan may board a lift.

Where those needs get relieved is [[Amenities]]. Plans are rides on [[Lifts]] and runs along [[Trails]]; [[Skiing]] carries out each run.

The long spec is [[Guests Spec]]. It lags the code in places (the action table predates the bar and the food court); trust the package when they disagree, and update the spec when a behavior changes.

## Log

- 2026-10-01: Ski loop, rest, hunger, thirst, tickets, and need preemption are in. The planner is still growing one goal at a time.

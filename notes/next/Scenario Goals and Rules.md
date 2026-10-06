---
title: Scenario Goals and Rules
kind: plan
status: in progress
---

# Scenario Goals and Rules

What makes one scenario play differently from another, beyond the map. Builds on [[Scenario Metadata]]. Listed in [[Next Steps]].

## Decisions

- **Required goals win; bonus goals are extra.** A scenario lists required goals, and meeting all of them wins. It can also list optional bonus goals, which never block the win and are tracked for bragging rights (and later, unlocks).
- **Winning doesn't end the game.** A "Scenario complete" panel congratulates the player, and the resort keeps running as a sandbox. Goals stay visible, marked done, and bonus goals can still be met afterwards.
- **Rules are named switches that also shape the build menu.** A rule like "no grooming" is saved as a switch. The sim honours it, and the build menu hides the tools it forbids, so the player never sees an option they can't use. Placement code checks the switch too, so nothing slips through a hotkey or an old save.

## Goals

Each goal has a kind, a target, and optionally a deadline. All of them are checked once per day at rollover, plus "something built" when it's placed.

| Kind | Target | Reads |
|---|---|---|
| Rating | a resort rating held for N days in a row | the demand rating ([[Demand]]) |
| Guests | guests in one day, or a daily average over N days | `History` arrivals |
| Season passes | passes sold this season | guest pass state |
| Cash | cash on hand at day end | `World.Cash` ([[Finance]]) |
| Debt free | nothing drawn on the credit line | `World.Cash` ≥ 0 |
| Built | a lift of some type, a lodge with a food court, a lift whose ends are near two named points | world objects |
| Open | the resort open on a date | `World.ResortOpen` ([[Calendar]]) |

A deadline is a date, or the end of season N. Missing a required goal's deadline loses the scenario, and so does bankruptcy (already tracked as `World.Bankrupt`). Losing shows a "Scenario failed" panel with Retry and Quit to menu; it doesn't continue as a sandbox.

Rating streaks need two changes: the demand rating must be saved (today it resets to 0.5 on every load) and recorded in each day's `History` sample.

## Rules

Switches set per scenario:

- **No grooming.** Snowcats and sheds are hidden and refused, and guests judge the resort on powder instead of corduroy ([[Grooming]]).
- **No cars.** Roads and parking lots are hidden and refused; guests arrive by train ([[Parking and Roads]]). Needs a train arrival, so this rule ships with [[Zermatt]].
- **Skiers only.** No snowboarders in the catchment ([[Guest Types]]).
- **No snowmaking** ([[Snowmaking]]).
- **Locals' resort.** A local-feel score that drops with crowding, high prices, and too few pass holders. Usable as a goal ("keep local feel above 60%") rather than a hard switch.
- **Climate.** Each scenario picks its own monthly [[Weather]] profiles, ideally fitted to real-world climate data. A southern-hemisphere resort also needs the season to run June to October.

Only rules a campaign scenario needs get built, in campaign order: no grooming for [[Asahidake]], no cars for [[Zermatt]], and so on.

## Steps

1. Done: **Data** (`world/goals.go`). `World.Goals` ([]`Goal`: kind, target, days for streaks, an optional deadline as the end of season N, bonus or required), `World.Rules` (named switches; `RuleNoGrooming` so far), and `World.GoalProgress` (per goal: met, the day it was met, current streak, best value so far, failed), all saved (`goals`, `rules`, `goal_progress`). Kinds built: lifts open, guests in one day, rating streak, cash, debt-free streak, and all required goals (for "everything within season one"); the rest of the table waits for a scenario that needs them. `Goal.Describe` gives the player's wording ("Welcome 1,500 guests in one day"). Progress resets when the goal list doesn't match it (an edited scenario). The resort rating moved from the demand system to `World.Rating`, so it's saved (`rating`; it used to reset to 50% on load) and recorded in each day's history sample.
2. Done: **Checking** (`sim/goals.go`). At each day rollover, after the daily sample, each goal not yet met or missed updates from the day: lifts open and not on hold, that day's arrivals, the rating (a streak), cash, debt free (a streak). "All required" is checked after the others. Met goals stay met; a season deadline (`SeasonCloseDate` of the season the scenario starts in, plus N−1) fails a goal not met by then. The scenario is won when every required goal is met (play carries on, and bonus goals keep counting), lost when a required goal is missed or the resort goes bankrupt; `World.Outcome` and the day are saved. Goal met and missed, won and lost go to the [[Event Feed]] (new kinds `EventGoal`, `EventScenario`, light blue). `sim.GoalStatus` words progress for the panel ("best 812 of 1,500", "3 of 7 days"). Checked headlessly on Boreal: easy goals win on day 3 with the bonus; a three-lift goal starting 20 May fails when the season ends.
3. **Goals panel.** A top-bar button opens the scenario's name, description, and goals with progress bars and deadlines. This is also where the description can be reread in-game. Win and lose panels on top.
4. **Editor.** A Goals tab in the Scenario details dialog adds, edits, and removes goals and toggles rules.
5. **Rules.** Start with no grooming: hide and refuse the cat shed and snowcats, and swap corduroy for powder in guest preferences. The others follow their scenarios.
6. **Unlocking.** Scenarios unlock in campaign order. A scenario unlocks when the one before it is won (required goals only). Progress lives in a small file next to the saves, not in any save.
7. **Boreal's goals** (decided 2026-10-06, see [[Boreal]]). Required: three lifts open at the end of a day; one day with guests in the thousands (the exact target, likely 1,000–2,000, set by a headless run of a reasonable three-lift Boreal); a rating of 70% or better for 7 days in a row. Bonus: meet them all within season one. No deadline: it's the tutorial. If three lifts can't reach thousands of guests in a day in the sim, that's a [[First Week Balance]] finding to report, not a reason to lower the bar quietly.

## Progression

Each scenario introduces a feature or two the earlier ones didn't need, so the campaign doubles as the tutorial for the whole game. [[Scenario Campaign]] lays out that order.

## Open questions

- Boreal's rating target: in the bare headless test resort (one lift, three plain trails, no lodge food or bar) the rating fell from 50% to about 35% within days, so 70% for a week needs a well-served resort, or rating tuning ([[First Week Balance]]). Check with the three-lift run.
- Boreal's exact guest target, from the headless three-lift run. One double chair with three trails averaged about 50 guests a day (2026-10-06), so thousands will need bigger lifts, more trail area, or more demand.
- Whether bonus goals unlock anything (a sandbox mode, a skin, a harder variant), or are just a record.
- How unlocking treats players who want to jump ahead: a "skip" option, or everything unlocked in the editor build only.

## Log

- 2026-10-01: Planned. Nothing implemented yet.
- 2026-10-02: Decided required plus bonus goals, keep playing after a win, and rules as switches that hide build-menu items. Wrote the goal kinds, the lose conditions, and the steps.
- 2026-10-06: Boreal's goals decided with the user: three lifts open, a best day of guests in the thousands, a 7-day rating streak at 70%; bonus: within season one; no deadline.
- 2026-10-06: Step 1: goal data, rules, progress, and the rating, all saved.
- 2026-10-06: Step 2: the daily goal check, win and lose, events, saved outcome.

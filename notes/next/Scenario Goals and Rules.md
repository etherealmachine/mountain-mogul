---
title: Scenario Goals and Rules
kind: plan
status: planned
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
- **No snowmaking**, or a water budget ([[Snowmaking]]).
- **Locals' resort.** A local-feel score that drops with crowding, high prices, and too few pass holders. Usable as a goal ("keep local feel above 60%") rather than a hard switch.
- **Climate.** Each scenario picks its own monthly [[Weather]] profiles, ideally fitted to real-world climate data. A southern-hemisphere resort also needs the season to run June to October.

Only rules a campaign scenario needs get built, in campaign order: no grooming for [[Asahidake]], no cars for [[Zermatt]], and so on.

## Steps

1. **Data.** `world.ScenarioGoals`: a list of goals (kind, target, days, deadline, required or bonus) and a set of rule switches, saved with the scenario. Progress per goal (met, met on which day, current streak) is saved in player saves. Save the demand rating and add it to the daily sample.
2. **Checking.** A sim step at day rollover updates each goal's progress, then decides won, lost, or still playing. Wins and losses go to the [[Event Feed]].
3. **Goals panel.** A top-bar button opens the scenario's name, description, and goals with progress bars and deadlines. This is also where the description can be reread in-game. Win and lose panels on top.
4. **Editor.** A Goals tab in the Scenario details dialog adds, edits, and removes goals and toggles rules.
5. **Rules.** Start with no grooming: hide and refuse the cat shed and snowcats, and swap corduroy for powder in guest preferences. The others follow their scenarios.
6. **Unlocking.** Scenarios unlock in campaign order. A scenario unlocks when the one before it is won (required goals only). Progress lives in a small file next to the saves, not in any save.
7. **Boreal's goals.** Open the resort, reach a set number of guests in one day, and hold a decent rating through a weekend; a bonus goal for a second lift.

## Progression

Each scenario introduces a feature or two the earlier ones didn't need, so the campaign doubles as the tutorial for the whole game. [[Scenario Campaign]] lays out that order.

## Open questions

- Exact targets for Boreal, which need playtesting alongside [[First Week Balance]].
- Whether bonus goals unlock anything (a sandbox mode, a skin, a harder variant), or are just a record.
- How unlocking treats players who want to jump ahead: a "skip" option, or everything unlocked in the editor build only.

## Log

- 2026-10-01: Planned. Nothing implemented yet.
- 2026-10-02: Decided required plus bonus goals, keep playing after a win, and rules as switches that hide build-menu items. Wrote the goal kinds, the lose conditions, and the steps.

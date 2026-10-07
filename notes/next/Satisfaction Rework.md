---
title: Satisfaction Rework
kind: plan
status: in progress
---

# Satisfaction Rework

Rebuild [[Satisfaction]] the way management games do it: stats change every tick and on events, satisfaction is a mood that drifts toward a target, thoughts are read-only reports of what changed a stat, and the resort rating is the day's average satisfaction. Each service becomes a set of stat effects, which gives new services (the [[Rental Shop]], door queues, amenity quality) a single place to plug in. Ranked first in [[Next Steps]]; it absorbs [[Patrol Day]] step 7 and changes how the thirst and exhaustion items are measured.

## Why

Read on 2026-10-07 against RollerCoaster Tycoon, Parkitect, Planet Coaster, and Planet Zoo (recalled from memory, not checked against sources). All four share a shape. Needs (hunger, thirst, energy, toilet, nausea) drain every tick and are restored by services. Happiness moves gradually toward a target that unmet needs pull down. Events (a ride, a purchase, a dirty path, a price that's too high) change stats directly. A thought is a short, deduplicated report of a condition or event, and it expires. Across the park, thoughts are counted and ranked, and that's the player's main diagnostic. Leaving is driven by stats, and a normal tired exit isn't a mark against the park.

What the sim does today, and where it differs:

- **Thoughts repeat while a condition holds.** `Guest.AddThought` only skips a repeat inside `ai.ThoughtTTL` (12 sim seconds, about 4 clock minutes), and `skiing.go` adds hungry, thirsty, tired, exhausted, and too-expensive every tick while the condition is true. One long thirsty stretch counts dozens of times. This is most of the "about 35 thirsty thoughts per visit" in the Guests section of [[Next Steps]].
- **Each satisfaction change has its amount hard-coded where it happens**: fall −0.10 and avalanche fall −0.20 (`skiing.go`), tree hit −0.15, injury −0.25 or −0.15, no one came to help −0.30, long line −0.08 twice (`simulation.go`), and no lodge −0.06 (`goap/planner.go`).
- **`ai.ThoughtSatisfactionWeight` is used only by the charts, and it disagrees with the sim.** It gives exhausted −0.15, hungry and thirsty −0.08, tired −0.05, and −0.20 to too expensive, no ticket window, lifts closed, and nothing for me. None of those change satisfaction in the sim. The thought charts weight counts by this table, so they rank problems by impacts guests never feel.
- **The exit chart shows each guest's last thought, not why they left.** Exhausted fires just before most guests go, so it tops the chart. That's most of "everyone leaves exhausted". Guests skiing two to three hours past close is a separate, real bug.
- **The planner both writes stats and adds thoughts.** `pickPlan` takes −0.06 from satisfaction on every replan where `Rest` fails, even when the thought is suppressed. It also calls `a.AddThought` directly instead of `Simulation.addThought`, so "needs a lodge", "lifts closed", "nothing for me", and "no ticket window" never reach the daily thought counts.
- **Needs don't affect mood.** A thirsty guest with no reachable bar is exactly as satisfied as one who just had a drink. Services matter only because they keep guests on the mountain.
- **Mood only drifts while skiing.** Waiting in line, riding, walking, and sitting in the lodge leave it unchanged.
- **The rating is a moving average over departing guests** (α = 1/70 in `demand.go`), with no daily figure for scenario goals or the player to read.
- **The [[Guests Spec]] thoughts table is out of date.** It lists `ThoughtLovingALift` (+0.10 on a first ride) and `ThoughtTiredOffPiste`, and neither exists in code.

## Decisions

Made with the user on 2026-10-07:

- **Keep the stats we have**: Energy, Hunger, Thirst, Patience, Balance, and Satisfaction. No new needs.
- **Update stats the way these games do**: needs drain every tick, satisfaction drifts toward a target every tick, and events change stats once.
- **Thoughts are read-only reports.** Every thought reports an event or condition that changed a stat. Nothing reads a thought to change a stat, and no stat changes without its thought.
- **The rating is the day's average satisfaction**: the mean final satisfaction of the guests who left that day. Other factors may be blended in later.
- **Services are defined by their stat effects.** Work out how each service changes stats now, so new services slot into the same table.

Confirmed by the user on 2026-10-07 after review:

- **Unmet needs pull the mood target down.** Hunger and thirst below 0.25 (where the guest starts looking for a service) and patience below 0.15 lower the target, scaled by how far below the threshold they are. Energy doesn't: tired at the end of a good day is fine.
- **Mood drifts in every activity**: skiing, in line, riding, walking, and in a building. Terrain terms apply only while skiing.
- **Each departure records one reason**, separate from thoughts. Tired, hungry, thirsty, or closing time count as a normal end of the day and cost nothing. Hurt, gave up on lines, can't afford it, no ticket window, and nothing to ski are the bad exits.

## The model

**Stats, per tick.** Drains stay as they are today (rates in [[Guests Spec]]): Energy while skiing, Hunger on a clock, Thirst scaled by altitude and exertion, and Patience while in line, with Patience recovering on skis and lifts.

**Satisfaction, per tick.** One function, `Simulation.tickMood`, runs for every guest on the mountain:

    target = 0.5
           + terrain terms (skiing only: glades, scared in trees, groomed, ungroomed; today's values)
           − need pull (hunger, thirst, patience below their thresholds)
    target clamped to [0.25, 0.80]
    satisfaction += (target − satisfaction) × driftRate × dt

**Events, once.** A single table, `ai.Effects [ThoughtKindCount]ai.Effect`, holds each event's stat deltas (Satisfaction, Energy, Patience) and the thought it reports. `Simulation.applyEvent(a, kind, context...)` applies the deltas, adds the thought, and counts it. It's the only code that writes event deltas, and `ThoughtSatisfactionWeight` goes away. The table's starting values are today's hard-coded amounts. The avalanche fall gets its own kind instead of being a `ThoughtFell` worth twice as much.

**Thoughts.** Two shapes, both reports:

- *Event thoughts* (fell, hit a tree, injured, long line, served a meal) are added once, when the event happens.
- *Condition thoughts* (hungry, thirsty, tired, too expensive, scared in trees, loving glades) are added when the condition starts and stay current while it holds. They're counted once per episode, and clear when the condition ends, with a margin so they don't flicker (for example, thirsty clears above 0.25). Each guest keeps a bitmask of the conditions that are active.

The six-entry ring stays for the follow panel. The daily "Guest thoughts" chart ranks by count × the table's satisfaction delta. For condition thoughts, the delta is the target pull they report.

**Departure.** `Guest.DepartReason` is set when `GoHome` wins. The "Exit thoughts" chart becomes "Why guests left".

**Rating.** `History` adds up each departing guest's final satisfaction for the day. At rollover, the day's average becomes `World.Rating` and that day's rating sample. If no one left that day, the rating stays where it was. [[Demand]] reads `World.Rating` as it does now. A 7-day 70% streak in [[Scenario Goals and Rules]] then reads straight off the daily samples.

## How services affect stats

Today, plus what each one gets in step 6:

| Service | Today | In this plan |
|---|---|---|
| [[Food Court]] | Hunger → 1, and rest (Energy and Patience → 1) | A "that hit the spot" event (+). Clearing hunger also lifts the target pull. |
| [[Bar]] | Thirst → 1 | Same, for thirst. |
| [[Lounge]] | Energy and Patience → 1 | A "good to sit down" event (+). Clears the patience pull. |
| [[Tickets]] | Lets the guest ride; none reachable sends them home | Missing: a bad departure reason, not a hit to satisfaction. |
| [[Lifts]] | Lines drain Patience; long-line events | Unchanged, moved into the table. |
| [[Grooming]] | Terrain term for guests who prefer groomed snow | Unchanged. |
| [[Ski Patrol]] | No effect | A rescue event whose delta scales with how long help took, replacing [[Patrol Day]] step 7. Abandoned stays −0.30. |

Later rows (not in this plan): a wait at the door, price against the guest's budget ("good value", "overpriced"), and amenity quality. Each is a new row in the table, not a new code path.

## Steps

1. Done: **One effects table.** `ai.Effect` and `ai.Effects` mark each thought as an event or a condition and hold its satisfaction amount. `Simulation.applyEvent` is the only place event deltas are applied. The planner no longer touches the guest: `ai.Plan.Blocked` lists what kept a goal from planning, and the sim turns that into conditions (`setBlocked`, also on the lift-ride lookahead, which used to drop them). `ThoughtSatisfactionWeight` is gone, and both thought charts rank by count, as RCT does, instead of count × weight. Changed from the plan: an avalanche is its own event ("an avalanche knocked me over!", −0.10) followed by the usual fall or injury, instead of a fall worth double. A tree hit (−0.15) now stacks with the injury it causes (−0.25), where before only the injury counted. "This corduroy is perfect" gets +0.05 so it reports something; [[Snow Tastes]] replaces it with run events. Only the spawn value, the reset, `applyEvent`, and `tickMood` write `Satisfaction`.
2. Done: **Condition thoughts.** `Guest.Conditions` holds the active conditions. `Simulation.setCondition` adds the thought and counts it once when a condition starts. `holds` clears it with a margin: needs at 0.15 on and 0.25 off, trees at 0.30 cover on and 0.20 off. The ring no longer skips repeats, and a held condition stays the current thought in the follow panel. Changed from the plan: "I need a break" now means low energy only, and low patience gets its own condition, "I'm sick of waiting around".
3. Done: **Mood in every activity, with needs.** `Simulation.tickMood` runs for every guest on the mountain each tick, including those being carried by patrol. The target is 0.5, plus the grooming pull from the last skiing tick, plus each active condition's pull, kept between 0.15 and 0.80 (the floor was 0.25). Pulls are flat while a condition holds, not scaled by depth: hungry, thirsty, impatient, and needs a lodge −0.10 each, glades +0.12, scared in trees −0.18. The grooming pull (+0.15 or −0.08 for guests who prefer groomed snow) still has no thought; [[Snow Tastes]] step 2 gives it one.

   Checked headless on the user's latest save (one lift, one trail, a lodge; 5 days, 112 visits, same seed before and after). Thoughts per visit fell from about 35 to 12: thirsty 3.60 → 0.49, too tired to ski 6.70 → 0.98, need a break 2.61 → 0.45, and the new "sick of waiting" 0.89. Average final satisfaction on the busy day fell from 0.44 to 0.35, because unmet needs and long waits now cost something. Exit thoughts now show "the lifts are all closed" for 20% of guests, since the last thought is no longer the most recently repeated one; step 4 replaces exit thoughts with departure reasons. Arrivals in that save stop after the third day in both versions; that isn't caused by this change.
4. **Departure reasons.** Add `Guest.DepartReason`, set it wherever `GoHome` wins, and record it in `History`. Replace the "Exit thoughts" chart with "Why guests left". Check: on Boreal, the share of normal end-of-day exits, and report what's left of the exhaustion item. Skiing past close stays its own bug.
5. **Daily rating.** Add a per-day satisfaction sum and count to `History`. Set `World.Rating` at rollover and drop `ratingEMAAlpha`. Saves break, which is fine before release. Check: the rating chart shows one point a day and demand follows it.
6. **Service events.** Add the rows from the services table: meal, drink, rest, and patrol response time. Check: on Boreal over two days, rescued guests end above abandoned ones, and fed guests end above hungry ones.
7. **Docs.** Rewrite "Satisfaction, Rating, and Thoughts" in [[Guests Spec]] from the code. Update [[Satisfaction]], [[Hunger]], [[Thirst]], [[Energy]], [[Patience]], and [[Demand]]. Close [[Patrol Day]] step 7.

Each step builds with `go build` and `go vet` and is judged in a headless Boreal run. No Go tests.

## Open questions

- How strong the need pull is compared with terrain and events. Set by eye in step 3.
- Whether the daily rating needs smoothing across days so demand doesn't swing. Start without it.
- Whether guests still on the mountain at rollover count toward that day (they shouldn't exist once skiing past close is fixed).
- Whether positive events beyond services earn a row: a first ride on a new lift, a powder run, a long run without falling.

## Log

- 2026-10-07: Planned with the user after comparing the satisfaction code with RollerCoaster Tycoon, Parkitect, Planet Coaster, and Planet Zoo. Ranked first.
- 2026-10-07: The user confirmed the three proposals (need pull, mood in every activity, departure reasons).
- 2026-10-07: Steps 1–3 built: the effects table, condition thoughts, and mood in every activity. Thoughts per visit about 35 → 12 on the user's save.

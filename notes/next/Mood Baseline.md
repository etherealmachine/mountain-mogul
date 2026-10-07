---
title: Mood Baseline
kind: plan
status: done
---

# Mood Baseline

Let a good day add up. Each guest gets a baseline, the 0.5 that their mood drifts toward today. A good run raises it, and each repeat of the same run raises it by less. A resort with varied, satisfying terrain then sends guests home happier than one with a single run lapped all day. Builds on [[Satisfaction Rework]], and shares its repeat counting with [[Snow Tastes]] step 5.

## Why

Found 2026-10-07 after [[Lift Unloading]] fixed falls on the user's Boreal save: 1.5 great runs a visit, and still a rating of 0.34–0.41 ([[Next Steps]], Mood can't climb past about 0.5). `tickMood` drifts satisfaction toward 0.5 plus the terrain's pull plus each active condition's pull, closing 0.6% of the gap per sim second (about a game hour). A great run is a one-off +0.04 that fades within that hour, so every guest ends the day near 0.5 minus their pulls, whatever their day was like. RollerCoaster Tycoon avoids this because rides raise a guest's happiness target, not just their current happiness.

## Decisions

From the user, 2026-10-07:

- **A good run raises the baseline**, not only the current mood.
- **Repeats of the same run raise it by less each time,** counted both per trail and per lift. The same trail decays faster and the same lift slower, so a high rating needs several lifts as well as several trails; lots of trails off one lift can't fake it.
- **Bad experiences lower the baseline.**
- **No carry-over between a regular's visits** for now.

## The model

- **Baseline.** `Guest.Baseline` starts at 0.5 on arrival and replaces `moodBaseline` in the target: baseline, plus terrain pull, plus condition pulls, clamped as now. It's the guest's memory of the day: it doesn't decay, and it's cleared when they go home.
- **The table.** `ai.Effect` gains a `Baseline` field, so any thought can move the baseline as well as the current mood. `applyEvent` applies both, and the baseline is clamped to [0.15, 0.85].
- **Raising it.** A great run raises the baseline by `0.04 × 0.6^t × 0.75^l`. Here t is how many great runs the guest has already had on that run's main trail today, and l is how many on any trail off the same lift. The immediate +0.04 event stays as a moment of joy that fades. The limits that follow:
  - **One trail lapped all day:** the baseline reaches at most 0.5 + 0.04 / (1 − 0.45) ≈ 0.57.
  - **One lift, however many trails:** the lift factor alone limits it to 0.5 + 0.04 / (1 − 0.75) = 0.66.
  - **Several lifts,** each with a couple of good trails: guests can reach the 0.85 cap.
- **Lowering it.** Bad experiences carry a baseline drop on their `ai.Effects` row, at full strength each time. Starting values: injured −0.05, hurt and going home −0.03, abandoned −0.10, slow patrol −0.03, too much for me −0.02, caught in an avalanche −0.03. Falls (on the run or getting off the lift) stay momentary.
- **Counting repeats.** `Guest.RunCounts` counts, per visit, every run and every great run by trail and by lift (a small slice, like `RidenLifts`). A run's lift is the one the guest unloaded from before it, recorded at unloading. [[Snow Tastes]] step 5 (boredom) reads the same counts.
- **The thought.** The great-run thought stays the same. Its first time on a trail could read "what a great run on X!" and later ones "another good one on X". The effect is what changes.

## Steps

1. Done: **Baseline.** `Guest.Baseline` starts at 0.5 (`baselineStart`) and replaces the fixed 0.5 in `tickMood`'s target. It's kept within 0.15–0.85, and the target's ceiling rose to 0.90 so the baseline's top shows. `Satisfaction` and `Baseline` are now saved for guests on the mountain (`sat`, `base`). Before, satisfaction reloaded as 0 for anyone on the hill.
2. Done: **Experiences move it.** `ai.Effect.Baseline`: injured −0.05, hurt and going home −0.03, abandoned −0.10, slow patrol −0.03, too much for me −0.02, caught in an avalanche −0.03, and great run +0.04. `applyEventScaled` scales the great run by `0.6^t × 0.75^l`. Here t and l are this visit's earlier great runs on the same trail and off the same lift, from `Guest.TrailTally` and `Guest.LiftTally` (`world.CountRun`; a run's lift is the one unloaded from, `Run.LiftID`). Meals stay momentary, as in RollerCoaster Tycoon, where food mainly ends hunger and price against value is the lasting part (later).

   Checked headless on the regraded Boreal save, 4 days. With one trail, the final baseline was 0.540 on average, 0.558 for the median guest, and 0.571 at most, against a limit of about 0.57. Split into three trails off the one lift: 0.540 average and 0.590 at most, under the one-lift limit of 0.66. No world with two lifts exists to check the multi-lift case; that rests on the limits above.

   The rating only rose from 0.34–0.41 to 0.38–0.42, because guests leave with satisfaction about 0.39 against a baseline of about 0.55. At departure, 95% have "sick of waiting" and the exhausted condition active, and 81% thirsty (Boreal has no bar). The first two come from `tickResortClosed` zeroing every guest's patience after closing, as the lever for going home. Recorded in [[Next Steps]] as Closing zeroes patience; this baseline work can't show in the rating until that's fixed.
3. Done: **Docs.** [[Satisfaction]] and [[Guests Spec]].

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- Whether services lift the baseline. Meals stay momentary for now; a price-against-value judgment could move it later.
- Whether the baseline should carry over between a regular's visits, toward the Regulars item. Not for now.

## Log

- 2026-10-07: Planned with the user after the falls fix left Boreal's rating near 0.4.
- 2026-10-07: The user decided: bad experiences lower the baseline; repeats decay per trail (faster) and per lift (slower); no carry-over for regulars yet.
- 2026-10-07: Built: the baseline, its effects, and per-trail and per-lift repeats. The rating is held down by closing time zeroing patience (Next Steps).
- 2026-10-07: With closing no longer zeroing patience, the regraded Boreal save rates 0.46–0.49 (satisfaction at exit 0.47 against a baseline of 0.54); 72% of guests still leave thirsty, since it has no bar.

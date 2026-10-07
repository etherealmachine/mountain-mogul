---
title: Mood Baseline
kind: plan
status: planned
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

1. **Baseline.** Add `Guest.Baseline`, used by `tickMood`, starting at 0.5 and reset on departure. Behavior is unchanged at this point.
2. **Experiences move it.** Add `Effect.Baseline` and the bad-experience values. Add per-trail and per-lift run counts, and raise the baseline in `judgeRun` with both decays. Check, headless on the regraded Boreal save (one lift, one green run): the rating should rise from about 0.4 toward the one-trail ceiling, about 0.57. Also check a test world with several trails off one lift (expecting it to stop near 0.66) and one with two lifts (expecting it higher).
3. **Docs.** Update [[Satisfaction]] and [[Guests Spec]].

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- Whether services lift the baseline (a great meal) or stay momentary.
- Whether the baseline should carry over between a regular's visits, toward the Regulars item. Not for now.

## Log

- 2026-10-07: Planned with the user after the falls fix left Boreal's rating near 0.4.
- 2026-10-07: The user decided: bad experiences lower the baseline; repeats decay per trail (faster) and per lift (slower); no carry-over for regulars yet.

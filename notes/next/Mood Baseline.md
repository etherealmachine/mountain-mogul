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
- **Repeats of the same run raise it by less each time.**

## The model

- **Baseline.** `Guest.Baseline` starts at 0.5 on arrival and replaces `moodBaseline` in the target: baseline, plus terrain pull, plus condition pulls, clamped as now. It's the guest's memory of the day: it doesn't decay, and it's cleared when they go home.
- **Raising it.** A great run raises the baseline by `greatRunLift × repeatDecay^n`, where n is how many great runs the guest has already had on that run's main trail today. The immediate +0.04 event stays as a moment of joy that fades. Starting values: lift 0.04, decay 0.6, baseline capped at 0.85. On one trail lapped all day, the baseline can reach at most 0.5 + 0.04 / (1 − 0.6) = 0.60. Each different trail adds up to another 0.10, so three or four good trails can carry a guest to 0.8.
- **Counting repeats.** `Guest.RunCounts`, per trail per visit (a small slice, like `RidenLifts`), counts every run on the trail, with great runs counted separately. [[Snow Tastes]] step 5 (boredom) reads the same counts.
- **The thought.** The great-run thought stays the same. Its first time on a trail could read "what a great run on X!" and later ones "another good one on X". The effect is what changes.

## Steps

1. **Baseline.** Add `Guest.Baseline`, used by `tickMood`, starting at 0.5 and reset on departure. Behavior is unchanged at this point.
2. **Great runs raise it.** Add per-trail run counts, and raise the baseline in `judgeRun` with the decay. Check, headless on the regraded Boreal save (one green run): the rating should rise from about 0.4 toward the one-trail ceiling. Check a resort with three runs at different levels too, expecting a higher rating than Boreal.
3. **Docs.** Update [[Satisfaction]] and [[Guests Spec]].

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- Whether bad experiences lower the baseline too: an injury, being left by patrol, slow patrol, a run that was too much. Falls probably stay momentary.
- Whether services lift the baseline (a great meal) or stay momentary.
- Whether repeats decay per trail or per lift, since one lift may serve several trails.
- Whether the baseline should carry over between a regular's visits, toward the Regulars item.

## Log

- 2026-10-07: Planned with the user after the falls fix left Boreal's rating near 0.4.

---
title: Star Ratings
kind: plan
status: in progress
---

# Star Ratings

Each guest leaves one to five stars when their car leaves the map, with a line saying why. The resort rating becomes the average star rating. Each star is a level the resort has to reach, not a cut-off on a score, so the player can see what earns each one and what's missing. There's no score behind the stars: the levels come from the guest's moments by class (dealbreakers, letdowns, annoyances, highlights, extras), in [[Scoreless Rating]].

## Decisions

From the user, 2026-10-09:

- **Stars are left per guest at departure**, so the rating can be read review by review.
- **Each star is a level the resort reaches**:
  - ★ a dealbreaker: turned away, hurt with no one coming, gave up.
  - ★★ skied, but with a letdown (a need unmet past its grace period, nothing to their taste), three annoyances, or no highlight.
  - ★★★ no letdown, fewer than three annoyances, at least one highlight. **Boreal's goal is 3★.**
  - ★★★★ needs more than skiing and the basics: shopping and other activities (to be worked out).
  - ★★★★★ isn't a level: it's reached only from 4★ with excellent service (the quality axis below).
- **Quality adds up to 1.2 stars** on top of the level: the average quality of the services the guest used, 0–1, times 1.2, capped at 5★. Excellent service is worth a level up: a 2★ guest at excellent places leaves 3.2★. Lift attendants count as a service (parking attendants later), so a guest who used none, at quality 0, is rare ([[Scoreless Rating]]).
- **4★ and 5★ are for later goals.** Boreal has no 4★ level, though excellent service can carry a 3★ guest to just over 4★; Kirkwood and later scenarios have the levels.
- **No satisfaction score** ([[Scoreless Rating]]). Guests track their good and bad moments; the star comes from the levels, not from a weighted sum.

## The model

**The star a guest leaves** is the highest level whose conditions they met, checked from what happened to them during the visit:

- **Moments**: the level is set by the guest's moments by class, three annoyances making a letdown ([[Scoreless Rating]]).
- **Quality**: plus 1.2 × the average quality of the services they used.
- **4★**: open below. **5★**: 4★ plus excellent service.

**The review line** names the moment that set the star: the dealbreaker, the letdown, the annoyances, or the best highlight ([[Scoreless Rating]]).

**The resort rating** is the day's average stars, shown as e.g. 2.8★. Demand and the ticket-price premium take (stars − 1) / 4 so they stay on 0–1.

**Goals**: the rating goal becomes a star target (`GoalRating` with Target 1–5), described as "Keep the rating at 3★ or better". With quality on top, a 3★ guest at today's quality (0.5) leaves 3.6★ and a 2★ guest 2.6★, so an average of 3★ is reachable without every guest at 3★.

**Readouts**: the Reviews tab, which replaced "Why guests left": the day's average stars and breakdown, and what set each review, ranked ([[Scoreless Rating]]). Each guest's review line is in their popup.

## Steps

1. **Stars per guest**: [[Scoreless Rating]] steps 1–3.
2. **Goals**: star targets in `Goal`, the goals panel and the editor; Boreal's goal set from a headless run.
3. Done: **Reviews tab**: breakdown and what set each review.
4. **4★**: once shopping and activities exist. 5★ follows from quality ([[Service Quality]]).

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- **The Boreal goal.** With quality, an average of 3★ no longer asks for every guest. Applying it to the measured days (1★ at 1.0, 2★ at 2.6, 3★ at 3.6): 2.87, 2.88, 2.92★, just under the goal; a rental shop would likely clear it.
- **4★**: which activities, and whether they're judged by what the guest wanted ("I wanted a sit-down lunch") or by what the resort has.

## Log

- 2026-10-09: Written up with the user: stars per guest as levels, Boreal's goal at 3★, 4★ and 5★ later.
- 2026-10-09: The user: drop the satisfaction score; track good and bad moments and rate from those ([[Scoreless Rating]]). Skiing worth the trip added as a need.
- 2026-10-09: Measured headless on the user's Boreal Goals Test save (three days, demand on), counting each unmet need at any moment: average 2.30, 2.29, 2.35★; 3★ 35%, 31%, 42%; 1★ 2–8 a day (bored before a full run). What holds guests at 2★, a day: renting in town 16–54 (mostly beginners: the map has no rental shop), skiing unmet 22–30 (bored beginners 14–16, the noted bug where novices lap one short green; not my skiing 7–14), thirsty 11–20, exhausted 5–9, chilled with no lounge 0–12, hungry 2–9. The ledger agrees with the levels: 2★ guests averaged 0.43–0.57, 3★ 0.65–0.69.
- 2026-10-09: Levels now come from moments by class ([[Scoreless Rating]]); renting in town is an annoyance, not a letdown, so the 2026-10-09 measurement's biggest cause would mostly drop to annoyances.
- 2026-10-09: Stars per guest and goals in stars built ([[Scoreless Rating]]). Boreal Goals Test: 3.02★ after one day, 3.12★ after two. The reviews panel is next.
- 2026-10-09: The Reviews tab replaces "Why guests left".

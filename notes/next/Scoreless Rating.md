---
title: Scoreless Rating
kind: plan
status: in progress
---

# Scoreless Rating

Remove the guest's satisfaction score. A guest's day is a collection of moments, each with a class: dealbreakers, letdowns, annoyances, highlights, and extras. The classes set the star they leave ([[Star Ratings]]), and the quality of the services they used adds to it. Every star comes from named moments, so "why is this a 2★?" always has a one-line answer. Replaces the ledger in [[Moments]], which replaced the baseline and drift in [[Mood Baseline]].

## Why

From the user, 2026-10-09: the ledger is the complicated part, and maybe the score isn't needed at all; track positive and negative guest events and rate from those. It builds on thoughts and conditions, making them binary and explainable, and there can be hundreds of unique moments for the player to discover.

- **Weights are what needs calibrating.** Every thought carries an amount in `ai.Effects`, and conditions cost by the clock hour. Each balance pass ([[Demo]] step 4, the 2026-10-08 calibration in [[Moments]]) has been tuning those numbers so an average lands near a target. With classes, nothing weighted decides the rating.
- **A score doesn't say why.** 0.567 is the sum of forty small amounts. A 2★ review names its letdown.
- **Levels already sort guests the way the ledger does.** On the Boreal Goals Test, guests who'd leave 3★ averaged 0.65–0.69 on the ledger and 2★ guests 0.43–0.57 ([[Star Ratings]] log), so dropping the score loses little.

## Moments

A moment is a thought that happened to the guest: an event, or a condition that held past its grace period. Each guest keeps a count per moment kind for the visit; the day keeps the same counts across guests (it already does, `ThoughtCountsToday`). Each moment kind has one class:

| Class | What it does to the day | Examples |
|---|---|---|
| **Dealbreaker** | the day is 1★ | turned away at arrival (lifts closed, nothing for me, no ticket window, priced out of a ticket), injured and no one came, caught in an avalanche, gave up on a run |
| **Letdown** | the day is at most 2★ | hungry, thirsty, nowhere to rest, chilled with no lounge, priced out of every place for a need, nothing here is my kind of skiing, skied this place to death, every lift line too long to join |
| **Annoyance** | three of them make a letdown ("too many little things") | a fall, a tree hit, a long line, a crowded run, too hard, overpriced, a shabby place, packed inside, a slow patrol, a fall unloading, rented in town |
| **Highlight** | 3★ needs at least one | a great run, first tracks, patrol arriving fast, a good meal when really hungry, a nice place |
| **Extra** | 4★ needs some (later) | a sit-down lunch, shopping, après, a lesson, the view from a summit restaurant |

Neutral moments (rented gear, warmed up, rested, a good-value visit) are counted and shown but don't move the star.

**The level.**

- 1★: any dealbreaker.
- 2★: any letdown, or three annoyances.
- 3★: no letdown, fewer than three annoyances, and at least one highlight. Without a highlight the day was fine but no more: 2★, "nothing special".
- 4★: 3★ plus extras (to be worked out with [[Star Ratings]]).
- There's no 5★ level: quality is the only way from 4★ to 5★.

A highlight being needed for 3★ replaces "skiing worth the trip" as a need: a guest fed and watered who never had a good run had an OK day, not a good one. Highlights don't stack: one is enough, and more only feed the review line and après.

**A grace period.** A condition becomes a moment only once it has held for a while (proposed 15 game minutes), so a guest thirsty on the way to the bar isn't marked down. Events (rented in town, turned away at arrival, a fall) count at once.

Not moments on their own: tired, cold, exhausted, impatient. They're the guest's state; failing to serve them (no lodge, no lounge) is the letdown.

**Rentals.** Coming without skis is a need for a small share of guests, mostly beginners and some intermediates (25%, 5%, and 0% by tier, down from 50%, 15%, and 5%). With no rental shop they rent in town: an annoyance, not a dealbreaker. A guest who can't rent at all and goes home shows in the Reviews tab as lost business but leaves no review, so it doesn't touch the rating. Advanced guests might want a demo shop instead, as an extra, later.

## Quality

A second axis, added on top of the level. A guest's quality is the average quality (0–1) of the services they used in the visit: meals, drinks, rests, rentals, warming up, après, and every lift ride, since lift attendants are a service (parking attendants and ticket windows later). It adds 0 to 1.2 stars (1.2 × quality), capped at 5★: excellent service is worth one level up, so a 2★ guest at excellent places leaves 3.2★. A guest who used no service has quality 0, but with lifts counted almost no one who skied gets there. Building quality is 0.5 everywhere today, worth 0.6; lifts have no quality yet and would start at 0.5; [[Service Quality]] makes both something the player sets through staffing and pay. The skiing itself doesn't count toward it.

## What a day looks like

- **1★**: "Drove up, the only lift with greens was closed, went home." "Hurt my knee on Kiss A Bear and waited 40 minutes for patrol." One dealbreaker, named.
- **2★**: "Good runs on Sunset Blvd., but I couldn't find a drink all afternoon." "Fell four times and waited forever at Lift 3." A real ski day with one thing the player could fix, named.
- **3★**: "Three great runs, lunch at the lodge, no complaints." At today's quality, 3.6★; at a well-run lodge with good lift crews, nearer 4.

## Discovery

New moments are cheap: a class and a text. So there can be hundreds:

- **Tied to a place**: first tracks on Sunset Blvd. and on Kiss A Bear are different entries, so each trail, lift, and building has its own.
- **Rare**: a bluebird powder day, the last chair, the first chair of the season, a 20-run day, skiing with a friend (with groups), wildlife from the lift.
- **Combinations**: a powder lover on a storm day, a beginner's first green from the top.
- **A moments catalogue** the player fills in as guests have them, like illnesses in Two Point Hospital or thoughts in Parkitect. Each entry shows its class once discovered, so it doubles as a guide to what earns stars.

## Review lines and readouts

- **The review line** names what set the star: the dealbreaker, the letdown, the annoyances ("fell four times and waited forever at Lift 3"), or at 3★ the best highlight ("4 great runs on Sunset Blvd.").
- **The Reviews tab** in the charts window, which replaced "Why guests left", is the day in aggregate: the average stars and the count at each level, then what set each review, ranked (a good day; each dealbreaker, letdown, or too many annoyances; nothing special; never got a run in), and the guests who went home without renting skis, as lost business. Each guest's own moments and review line are in their popup; that's all the detail there is.
- **Après**, which scores more after a good day, scales with the day's level instead of the score.

## What stays

- **Thoughts**, which become moments, and the day's counts and charts.
- **Conditions**, which become moments once past the grace period.
- **Departure reasons** (`DepartReason`), kept on each departure; the Reviews tab shows the no-rentals ones as lost business.
- **GOAP**, which never read the score.
- **Run verdicts** (`judgeRun`): great, too easy, too hard, crowded, first tracks, miserable.

## What goes

- `Guest.Satisfaction` and its save field; `scoreStart`.
- The `Satisfaction` amounts in `ai.Effects` (`internal/ai/types.go`). The table keeps `Condition` and gains the class.
- The hourly cost of conditions in `tickMood` (`internal/sim/skiing.go`).
- The charge for conditions still on at departure (`finishDeparture`).
- `applyEventScaled` and the repeat discount on great runs. Boredom already handles lapping one trail.
- `History.SatisfactionToday` and `DayRating` as an average score.
- `Guest.LastScore`, replaced by the stars the guest left last visit.

## Demand and saves

- **The resort rating** is the day's average stars. Demand and the ticket-price premium take (stars − 1) / 4, so their 0–1 inputs keep their shape. Recalibrating demand isn't part of this.
- **Saves**: the guest's score and last score go; moment counts and condition start times are saved for guests on the mountain; the last stars left are saved per guest. Old saves break, which is fine before release.

## Steps

1. **Moments and classes**: the class on each thought kind, moment counts per guest, the grace period on conditions, the level and quality at departure; History counts stars per day and the rating is the average. Saves.
2. **Remove the score**: `Satisfaction`, the amounts in `ai.Effects`, `tickMood`'s cost, the departure charge, `LastScore`; après from the level; the guest popup shows moments.
3. **Rentals**: smaller shares, mostly beginners; renting in town is an annoyance; no-rentals departures stay, with no review.
4. **Review lines**: from the moment that set the star.
5. **Goals and readouts**: [[Star Ratings]] steps 2 and 3.
6. **Docs**: [[Moments]] rewritten as moments and classes (or renamed Ratings), and the notes that link it checked.

Later: the moments catalogue, moments tied to a place, rare moments.

Each step builds with `go build` and `go vet` and is checked headless on the Boreal Goals Test: the star breakdown and the moments that set it. No Go tests.

## Open questions

- **Classes for the borderline moments**: too hard (annoyance, or a letdown for a novice?), a slow patrol after an injury, miserable snow all the way down.
- **Impatience.** Patience runs down mostly in lift lines. Whether running out of it is a letdown, or only "lines full" is.
- **Taste.** Great runs are scaled by taste today (a powder lover's powder run counts for more). With classes, a taste can only decide whether a run is a highlight at all, or add its own moment (first tracks already does).

## Log

- 2026-10-09: Written up with the user: no satisfaction score; needs met set the star, unweighted moments explain it.
- 2026-10-09: The user: great runs are a moment like any other, and moments count in the rating.
- 2026-10-09: The user: quality as an extra axis, added on top of the level, 0–1 scaled to 0–1.2 stars; services only.
- 2026-10-09: The user: no services means quality 0; lift attendants (and later parking attendants) are a service, so skiing without one is rare. Excellent service is worth a level up, and is the only way from 4★ to 5★.
- 2026-10-09: The user: needs get a grace period.
- 2026-10-09: Measured an unweighted good-against-bad rule on the Boreal Goals Test (three days): 6.1–7.0 good moments a guest against 0.85–0.90 bad. One star off when bad outnumber good touched no guest at 3★ and moved 4–6 a day from 2★ to 1★. How often a moment can happen acted as its weight, which led to classes.
- 2026-10-09: The user: the rating is the collection of moments, by class (dealbreaker, letdown, annoyance, highlight, extra); three annoyances make a letdown. Rentals: a need for a small share, mostly beginners; renting in town is an annoyance, not a dealbreaker; advanced guests might want a demo shop.
- 2026-10-09: Built steps 1–4 and the goal side of 5: classes in `ai.Effects` (the satisfaction amounts, `tickMood`'s hourly cost, the departure charge, `applyEventScaled`, and the great-run repeat discount removed); a grace period of a quarter clock hour on conditions (`momentGrace`); quality from every building visit and lift ride (lifts at `world.LiftQuality` 0.5); `world.Review` at the car, the day's average stars as `World.Rating` (demand reads `RatingShare`); review lines (`scene.ReviewLine`) in the guest popup and the day's recap; goals in stars; saves carry moments, quality, and run tallies. Rentals 25%, 5%, 0% by tier; renting in town is an annoyance. Left: the reviews panel and the moments catalogue.
- 2026-10-09: First run on the user's Boreal Goals Test save (rating starts at 3★, since the old rating isn't read): 3.02★ after day 1 and 3.12★ after day 2. Day 1: 107 reviews, 1★ 2, 2★ 58, 3★ 47; beginners 3.09, intermediates 2.96, advanced 2.78. Below 3★: thirsty 18, skied this place to death 14 (the bored-novices bug), no lounge 10, not my kind of skiing 4, hungry 3, too many falls 5, gave up 2, nothing special 2. Day 2: 3★ 55 of 106; skied to death 24, thirsty 11, not my kind of skiing 7.
- 2026-10-09: The user: keep no-rentals departures as a sign of lost business; they leave no review and don't count toward the rating.
- 2026-10-09: The user: "Why guests left" becomes Reviews, the aggregate; each guest's moments are in their popup. Built: the Reviews tab in the charts window (it replaced "Why guests left"): the day's average stars and count at each level in its heading, then a ranked list of what set each review (a good day; each dealbreaker, letdown, or too many annoyances; nothing special; never got a run in) and the guests who went home without renting skis, as lost business.
- 2026-10-09: Hunger and thirst don't use up the grace period while the guest's plan is taking them to food or drink (`seeingToIt`): guests in line or being served were being marked thirsty. Found balancing Christmas on the [[Season Calendar]].

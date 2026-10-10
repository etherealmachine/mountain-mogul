---
title: Boreal Tutorial
kind: plan
status: planned
---

# Boreal Tutorial

The tutorial's build order and the money that paces it, day by day: what the player builds, what it costs, what it earns, and what has to change in the economy to make it play out. Scenario: [[Boreal]]. Pacing rules: [[Economy Balance]]. Listed in [[Next Steps]].

## The build order

From the user, 2026-10-10. The player starts with one parking lot, sized for about half of a 1,000-guest day, and nothing else.

1. **Open.** Build a lift (California Cruiser, the beginner lift) and a ticket office. Run a day to test it and earn something.
2. **Grooming.** A cat garage with one cat. Watch a night's grooming.
3. **Patrol.** A patrol service with a snowmobile.
4. **Lunch.** A small lodge.
5. **Check the books.** Balance staffing and prices; make sure each day's net is positive.
6. **Grow.** Take out a loan and build the second lift (Accelerator, the main lift).
7. **Wait** a few days for revenue.
8. **Grooming for two lifts.** A second cat, and a bigger garage.
9. **Parking.** A second lot.
10. **Finish.** Borrow again or wait, then build the third lift (49er). The goals are met about then.

## Measured today

Each step built from the Boreal Goals Test save and run headless for 10 days from Dec 7 (8 ordinary days, Christmas and MLK Day), 2026-10-10. Stage 4's lodge is cut to 6 food court and 2 lounge tiles. Costs are what's built, including the starting lot ($150k).

| Stage | Built | Cost so far | Open-day costs | Ordinary day: visitors, net | Holiday: visitors, net | Stars |
|---|---|---|---|---|---|---|
| 1 | lot, Cruiser, tickets | $926k | $1.1k | 283, $15.2k | about 1,150, $51k | 2.7 |
| 2 | + garage (12 tiles), 1 cat | $1.13M | $2.6k | 290, $15.5k | about 1,110, $48k | 2.7 |
| 3 | + patrol, 1 snowmobile | $1.17M | $2.8k | 300, $15.1k | about 1,190, $51k | 2.7 |
| 4 | + small lodge | $1.44M | $3.6k | 285, $22.9k | about 900, $53k | 2.7 |
| 5 | + Accelerator | $3.09M | $4.9k | 393, $35.3k | about 1,315, $92k | 3.0 |
| 6 | + second cat | $3.24M | $6.1k | 370, $31.9k | about 1,280, $87k | 3.0 |
| 7 | + second lot | $3.39M | $6.1k | 384, $32.0k | about 1,225, $87k | 3.0 |
| 8 | + 49er, the full lodge, third cat, second snowmobile | $4.49M | $9.2k | 422, $34.5k | about 1,090, $97k | 3.1 |

What this shows:

- **Guests hardly depend on what's built.** One beginner lift with no food already draws 283 on an ordinary day and over 1,100 on a holiday; the finished resort draws 422. So the second lift pays back in about 130 days, and the third lift, the second lot, and the cats never do.
- **Grooming and patrol change nothing measurable**, in money or stars, so there's no reason to build them except that the tutorial says so.
- **Parking never fills.** 360 stalls held 1,236 guests at Christmas, so the second lot does nothing.
- **Early income is small against early costs.** At $15k a day, the garage and cat ($205k) take 13 days of income, and the small lodge ($270k) 18.
- **Running costs are tiny** (2–8% of revenue at first), so the books always look fine and step 5 has nothing to teach.

## Measured after steps 1–2

2026-10-10, after demand was decoupled from the season and made to follow what's built (6-day months; `terrainDraw` at `dailyVisitShare` 0.1). Six days from Dec 4: four ordinary days, Christmas (Dec 6) and MLK Day (Jan 3). The table before this counted every painted run for every stage, even runs no open lift served; now only the runs the running lifts serve count.

After the occupancy fix (below):

| Stage | Built | Runs served (green, blue, black cells) | Ordinary day: visitors (average), net | Christmas / MLK: visitors, net | Stars, ordinary / holiday |
|---|---|---|---|---|---|
| 1 | lot, Cruiser, tickets | 2 runs: 1,956 green | 221–482 (350), $13–27k | 1,125 / 635, $45k / $25k | 2.5 / 2.0 |
| 4 | + garage, cat, patrol, small lodge | the same | 261–432 (375), $20–36k | 1,278 / 1,017, $69k / $58k | 2.7 / 2.1 |
| 5 | + Accelerator | 8 runs: 4,971, 2,352, 2,521 | 568–1,101 (850), $52–93k | 2,077 / 1,302, $116k / $85k | 2.7 / 2.0 |
| 8 | + 49er, the full lodge, cats, second lot | 11 runs: 5,296, 2,885, 2,871 | 843–1,180 (1,075), $78–114k | 2,937 / 2,788, $206k / $188k | 2.9 / 2.25 |

What this shows:

- **Accelerator pays.** It more than doubles ordinary days (about 375 → 850) and adds about $45k a day net, so its $1.65M pays back in about five weeks of ordinary days at today's prices, sooner with holidays.
- **49er adds about 25% on ordinary days**, mostly through lift capacity easing crowds (49er's runs are nearly all served from Accelerator already).
- **Two lifts may meet the guest goal** (a day in the thousands): stage 5 drew 2,077 at Christmas. Fine by the user (2026-10-10): the targets can move, as long as the rating goal and three lifts stay and the money works.
- **The rating goal is far off.** 70% is 3.8 stars; ordinary days run 2.5–2.9 and holidays 2.0–2.25 at every stage. Christmas at the full resort: 1,125 of 2,937 went home fed up with lines.
- **Holidays overfill** until the tutorial's lots are smaller (Lots, below).

**The stage 4 dip, diagnosed** (2026-10-10). Before the fix stage 4 drew about 200 on ordinary days against stage 1's 270, with the same lifts and runs. Bisecting stages 2–4 put it on the lodge. Cause: demand's occupancy counted every guest on the mountain, the lodge included, against the smaller of lift and terrain capacity, and terrain capacity had become 196 guests once it counted only Cruiser's runs (this plan's step 2). Guests with lunch stay all day (about 150 on the mountain 10:00–14:00, against 60 without food, who go home by 11), so stage 4 ran at about 80% "full" and most arrivals stayed home. Fix (`sim.Occupancy`): the fuller of the lifts (everyone not indoors, against lift capacity) and the runs (only guests out on the snow, against terrain capacity). Stage 4 now draws about 375 against stage 1's 350. What made it hard to see: occupancy wasn't visible anywhere; `tools/stages ... -hourly` now prints it with what guests are doing and why they went home.

**Lots** (2026-10-10). Lots already fill: a car with no stall goes on to another lot or home, and the feed says "Cars turned away". The day's stats now count the guests turned away (`DailySample.TurnedAway`), and the day report shows "Turned away: lots full". The Goals Test's lots just rarely fill: guests come about 2.5 to a car and mostly stay till closing on ordinary days (the full resort peaked at 446 cars for 1,175 visitors), so its first lot (360 stalls) holds about 900 at once. At Christmas the full resort filled both lots (721 stalls; 213 turned away), and still saw 2,937 visitors because about half left early (1,125 fed up with lines, 477 done for the day), freeing stalls. So lots that fill for the tutorial is a matter of size: a starting lot of about 200 stalls holds about 500, which caps stage 4 holidays and makes the second lot worth building once Accelerator runs. That's set when `boreal.save` is rebuilt (step 7).

**With a 200-stall first lot** (`tools/stages … -stalls 200`, the first lot cut to its 200 stalls nearest the driveway), 2026-10-10, six days from Dec 4:

| Stage | Ordinary day: visitors, turned away, net | Christmas / MLK: visitors, turned away | Stars, ordinary / holiday |
|---|---|---|---|
| 1 Cruiser | 221–482, none (peak 168 cars), $13–27k | 1,125 / 635, none | 2.5 / 2.0 |
| 4 + lodge | 285–432, none, $23–36k | 1,220 / 991, 133 / 243 | 2.7 / 2.0 |
| 5 + Accelerator | 493–691, 309–593, $45–61k | 769 / 587, 2,539 / 2,727 | 2.7 / 2.4 |
| 7 + second lot | 710–1,037, none, $65–87k | 2,652 / 2,481, 164 / 426 | 2.6 / 2.15 |
| 8 + 49er, full lodge | 880–1,175, none, $82–114k | 2,192 / 1,788, 1,535 / 1,642 | 2.9 / 2.35 |

So the build order reads as planned: the lot first fills on holidays with the lodge, then every day once Accelerator runs (about 500 turned away on an ordinary day), and the second lot's $150k pays back in about 6 ordinary days. Two lifts and plenty of parking bring the crowds; 49er and the full lodge bring the stars up (2.6 → 2.9 ordinary) and keep guests longer, so fewer churn through on holidays. The rating goal (3.8 stars) is still well out of reach.

The day-by-day target below predates the 6-day months (Christmas is now day 6 from Dec 1, MLK Day day 9) and needs re-dating with step 7.

## The target

About 20 game days, Dec 1 to about Jan 10, with Christmas (day 10) and MLK Day (Jan 5, day 15) as paydays. Each step affordable within 1–3 days of the last, by revenue or a loan, as [[Economy Balance]] asks.

Cash is at the end of each day, after that day's building and net; days not shown just earn.

| Day | Date | Player does | Cost | Net that day | Cash |
|---|---|---|---|---|---|
| 0 | Dec 1 | starts with the lot, $700k cash, $1.5M credit at 12%; builds Cruiser and a ticket office | $620k | | $80k |
| 1 | Dec 1 | runs the day | | $35k | $115k |
| 2 | Dec 2 | garage (4 tiles) and one cat; watches the night's grooming | $100k | $35k | $50k |
| 3 | Dec 3 | patrol and a snowmobile | $40k | $35k | $45k |
| 6 | Dec 6 | small lodge (6 food, 2 lounge), after the day | $150k | $35k | $0 |
| 7 | Dec 7 | checks the books; borrows $1.3M; Accelerator | $1.3M on credit | $55k (lot full) | $55k |
| 10 | Dec 10 | Christmas: about 1,600 want to come, the lot holds about 500 | | $60k | $225k |
| 11 | Jan 1 | second cat and a bigger garage | $120k | $55k | $160k |
| 13 | Jan 3 | second lot | $150k | $85k | $150k |
| 15 | Jan 5 | MLK Day: about 1,100 fit, a day in the thousands | | $120k | $355k |
| 18 | Jan 8 | 49er, from cash (the last $200k of credit if the days run short) | $500k | $100k | $125k |
| about 20 | Jan 10 | three lifts open; 7 days at a 70% rating since about day 13 | | | |

Targets behind it (ordinary days):

| Built | Want to come | Fit in the lots | Spend per visitor | Net |
|---|---|---|---|---|
| Cruiser | 450 | 500 | $75 (tickets) | $35k |
| + lodge | 500 | 500 | $110 | $50k |
| + Accelerator | 800 | 500 | $110 | $55k |
| + second lot | 800 | 1,100 | $110 | $85k |
| + 49er | 1,000 | 1,100 | $110 | $100k |

Holidays bring about twice an ordinary day's crowd, up to what the lots hold.

## What has to change

1. Done 2026-10-10: **Guests follow what's built.** How many want to come should grow with the lifts and terrain (about 450 for one beginner lift, 800 with Accelerator, 1,000 with all three; today 280 to 420). This is [[First Week Balance]] step 2, and the biggest change: without it nothing after the first lift pays back.
2. Decided 2026-10-10 (the user): demand no longer divides by the season's length, and the season is 30 days as 6-day months. **The season's length was a demand lever** (the user's hunch, and it held): a guest's visits a season are spread over `world.SeasonDays`, so the 50-day season sets how many come each day. A 30-day season gives every day about 1.7 times today's crowd, which by itself closes most of the gap for the first lift. It can't make the crowd grow with what's built (change 1), and it shortens every scenario's calendar too. Decide this with the user before tuning anything else.
3. Done 2026-10-10, but for the lot size: **Lots fill.** A lot should take about one car a group for the day, so the starting lot (about 200 stalls) holds about 500 guests. Arrivals past that are turned away with "Lots full", which already logs; the day report should say how many. That's what makes the second lot worth building, and what Christmas teaches.
4. **Grooming and patrol pay off.** Without grooming, runs go to moguls and ice in 2–3 days, and the rating drops below 70% (the goal can't be met). Without patrol, an injured guest waits long enough to leave a 1★ review and an event. Then both earn their place, and step 2's "watch a night's grooming" is the lesson that day 3 got better.
5. **Prices**: day ticket $75 (from $60); food spend about $35 a visitor, about today's.
6. **Costs**, toward the target table:
   - Cruiser about $600k (today $754k; the fixed quad's $700k station fee is most of it)
   - a 4-tile garage and a cat about $100k (cat $80k, today $150k)
   - small lodge about $150k (today $270k for 6 food and 2 lounge tiles)
   - Accelerator about $1.3M (today $1.65M)
   - patrol ($40k), the second lot ($150k) and 49er ($506k) are about right
7. **Step 5 needs something to check.** Today the only staffing choice is a lift's line attendant, and running costs are a few percent of revenue. Either the lodge's staffing comes first ([[Service Quality]]: staff pools per room), or step 5 is prices and attendants only, with running costs nearer [[Vision]]'s 30–50%.
8. **Starting money**: $700k cash and a $1.5M credit line at 12% (the editor's Money tab, [[Scenario Editor]]). Interest on $1.5M is about $500 a day: small, but the loan should show on the books.

## Real time

Normal speed is now a game day in 2,560 s (43 minutes, [[Season Calendar]]), so the tutorial can't be played at normal: 20 days would take 14 hours. At fast (160 s a day) the 20 days take about 55 minutes, plus building time. So the tutorial should ask the player to watch the first morning at normal and then fast-forward, and the gap between normal and fast (16 times) may need a speed between them.

## Steps

1. Done: the season length (change 2): 6-day months, demand decoupled.
2. Done: demand grows with what's built (change 1): terrain served by skill and run count, with the rating (Measured after steps 1–2). Left: why stage 4 draws less than stage 1.
3. Done: turned-away guests counted and shown (change 3); the starting lot's size is set in step 7. Also done: the stage 4 dip (occupancy counted the lodge against terrain capacity).
4. Grooming and patrol effects (change 4).
5. Prices, costs and starting money (changes 5, 6 and 8).
6. Step 5's content (change 7).
7. Rebuild `boreal.save` as the start: one lot, $700k, the credit line, the goals.
8. Play it through at fast speed, and re-measure the stages against the target table.

## Measuring

`go run ./tools/stages "assets/scenarios/Boreal (Goals Test).save" <stage> <days>` builds a stage from the Goals Test save by removing what comes later, prices it, and runs it headless; it's how the table above was measured, and how each change should be re-measured.

## Open questions

- Does the tutorial let the player build out of order, or guide each step (grants, as in [[Economy Balance]])?
- Which dates should the tutorial open on: Dec 1, so Christmas is day 6 and MLK Day day 9?
- Settled (the user, 2026-10-10): no magic carpet in the demo; guest targets can move, but the 70% rating goal and three lifts stay, and it's fine if two lifts would meet the guest goal.

## Log

- 2026-10-10: Planned with the user: the build order. Each stage measured from the Goals Test save; guests barely follow what's built, and grooming, patrol and the second lot change nothing measurable. Target day-by-day timeline and the changes it needs.
- 2026-10-10: Steps 1–2 (the user): 6-day months; demand decoupled from the season (`dailyVisitShare` 0.1) and drawn by the terrain the running lifts serve, by skill and run count (`terrainDraw`). Re-measured: Accelerator triples ordinary days; stage 4 draws less than stage 1, undiagnosed.
- 2026-10-10: The stage 4 dip diagnosed and fixed (occupancy counted lodge guests against terrain capacity); full lots counted in the day's stats and report; re-measured. The user: no magic carpet in the demo, guest targets can move, the rating goal and three lifts stay.
- 2026-10-10: Measured with a 200-stall first lot: it fills on holidays with the lodge and every day with Accelerator; the second lot pays back in about 6 days. `tools/stages` gained `-hourly` and `-stalls`.

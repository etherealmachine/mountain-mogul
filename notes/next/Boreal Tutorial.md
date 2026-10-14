---
title: Boreal Tutorial
kind: plan
status: started
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

## Played through (2026-10-12)

The user's stages (2026-10-12): the player runs out of money and waits a day or two for each, and the last lift needs a longer wait, the credit line, or savings.

1. A beginner lift with two runs, grooming (one cat) and a ticket window.
2. A lodge with a food court, and ski patrol. Built with stage 1, it leaves very little money.
3. A second lift, with beginner and intermediate runs.
4. The third lift, with advanced runs where there are any.

`tools/tutorial` plays it headless: the tutorial's start from `boreal.save` (its cash, credit line and goals, one 300-stall lot) on the Goals Test's layout, which lines up with the tutorial's map. It takes out everything but the lot, then puts each stage back as soon as the money allows, at what the game charges: lifts, buildings by tile, cats and snowmobiles, and clearing the trees the tutorial's map has on each run ($100 a tree, about $200k of Accelerator's runs). Cruiser gets a second green, painted beside Kiss A Bear. Stage 1: Cruiser, Kiss A Bear and the second green, a 4-tile garage with a cat, tickets ($755k). Stage 2: the lodge with 6 food court and 2 lounge tiles, patrol with a snowmobile ($189k). Stage 3: Accelerator and every green and blue ($1.52M). Stage 4: 49er and the blacks ($527k). Stage 5: the lodge to 16 food court and 8 lounge, the garage to 12 tiles, two more cats ($422k).

**What it found.**

- With `boreal.save`'s $1M cash and $1M credit, stages 1 and 2 go up on day 0 and leave $56k: as wanted. But stage 3 waits until day 21 (cash and credit together), and paying cash only it never comes before the season ends. Earnings with one lift are $10–20k on an ordinary day and about $55k on a holiday.
- **The rating goal couldn't be met.** Even the full resort ended ordinary days at 2.5–2.8★ and holidays at about 2.1★. Cruiser's runs are never great runs (about 38 m of vertical against the 40 m minimum), so with one lift every review is 2★ (2.6 with service quality). On the full resort two thirds of 2★ reviews had a highlight, mostly great runs, and most of those were pulled down by three small annoyances, mostly "it's packed in there" from the small lodge.
- The guest goal (1,000 in a day) falls on the first holiday, Christmas, with only Cruiser open.

**Changed.**

- The credit line is $1.4M (`boreal.save`; the user chose it).
- Highlights offset annoyances: annoyances make a letdown only when three more than the day's highlights ([[Scoreless Rating]]; the user chose to make 3★ reachable). The full resort with the small lodge then reaches about 2.65★; with the full lodge and three cats, 2.95–3.17★ on ordinary days.

**Played through after** (two seeds, the same day by day):

Builds come at the end of the day shown, from what it took. The calendar is the Goals Test's, opening Dec 4; `boreal.save` starts on another date, so its holidays fall on other days.

| Day | Date | Built after it | Cash after | Note |
|---|---|---|---|---|
| 0 | before opening | stages 1 and 2 | $56k | one lift: 250–330 visitors, $15–20k a day |
| 3 | Dec 6 (Christmas) | stage 3, on credit | −$1.37M | about 1,070 visitors, the guest goal met |
| 11 | Feb 2 | stage 4 | −$1.39M | an 8-day wait; three lifts open |
| 17 | Mar 2 | stage 5 | −$1.39M | another 6 days |
| 19 | Mar 4 | | −$1.26M | 3.16–3.17★: won |

So stages 1–2 come at once, stage 3 waits for Christmas's takings, stage 4 waits 8 days for the credit line to free up, and the rating needs the lodge and grooming grown on top (another 6 days). The credit line is nearly all drawn at the win.

**Demand retuned** (the user, 2026-10-12: one lift with two beginner runs shouldn't bring 1,000+ guests). On Christmas at one lift, no more than about 300 were on the mountain at once, but guests fed up with lines left and others came in their place (729 left fed up). The terrain's draw now saturates more slowly (half at 3,000 suitable cells and 6 runs, from 1,000 and 2) and `dailyVisitShare` doubled (0.1 → 0.2), so a small hill draws about half what it did while the full resort's crowds hold (its lot fills either way). Stages 1–2 now draw 100–210 on an ordinary day (from 250–330) and 680–770 on a holiday (from about 1,100), and net about $0–10k on an ordinary day: one lift with a lodge and patrol about breaks even.

| Day | Date | Built after it | Cash after | Note |
|---|---|---|---|---|
| 0 | before opening | stages 1 and 2 | $56k | 100–210 visitors on ordinary days, about 700 at Christmas |
| 6 | Jan 3 (MLK Day) | stage 3, on credit | −$1.37M | waited for two holidays' takings |
| 9 | Jan 6 | | | about 1,600 visitors: the guest goal met, with two lifts |
| 14–15 | Feb 5–6 | stage 4 | −$1.33M to −$1.39M | an 8–9-day wait |
| 16 or 22 | | stage 5 on day 20 (seed 2) | | won: seed 1 met 3★ on a quiet day (567 visitors, 3.06★) with two lifts and the small lodge, so three lifts on day 16 won; seed 2 needed stage 5 |

So the guest goal now needs the second lift, stage 3 waits six days instead of three, and the win comes on day 16–22.

**A 3-day rating** (the user, 2026-10-12: the rating is the average of the last three days' stars, so one quiet good day can't meet the goal). With holidays on the 3rd and 6th of every 6-day month, every three days hold a holiday, and holidays rate 2.2–2.4★ at every stage, so the finished resort's best was 2.82★ (ordinary days 2.9–3.1★): the tutorial couldn't be won by day 30.

**What a player can do about holidays** (the finished resort with stage 5, three days to Christmas, one seed each; single days vary by about ±0.15★):

| Player does | Ordinary days | Christmas | 3-day rating after Christmas |
|---|---|---|---|
| nothing more | 2.88, 3.00 | 2.44 | 2.77 |
| a line attendant at every lift | 2.96, 2.96 | 2.78 | 2.90 |
| each lift upgraded a step | 3.02, 2.98 | 2.67 | 2.89 |
| $90 day ticket | 2.98, 3.06 | 2.36 | 2.80 |
| attendants and upgrades | 3.01, 3.03 | 2.51 | 2.85 |
| attendants, upgrades, $90 | 2.95, 3.01 | 2.77 | 2.91 |
| attendants and $90 | 3.05, 3.13 | 2.51 | 2.90 |

What sinks a holiday is the 1★s (582 at Christmas, against about 85 on an ordinary day), and most are guests who never got a run: every lift line looked too long (185), the lodge was packed (about 60), they'd had to rent skis in town (33), or nothing named (about 80). Each one's stall frees a space for the next car, so about 1,350 come through a 300-stall lot while 2,800 more are turned away. Attendants cut the 1★s to about 240; nothing tried gets a holiday past about 2.8★, or three days past about 2.9★. A price rise doesn't thin the crowd: demand is several times what the lot holds.

**Guests give their first ride a chance** (the user, 2026-10-12, first of the tools above). A guest who hasn't ridden yet today joins a line of up to 90 clock minutes (`world.FirstRideMaxLineWait`), not 30: they came to ski, so they queue and grumble ("this line is way too long", an annoyance) instead of going home without a run. On the finished resort at Christmas: 2.44 → 2.92★ (2.99 with attendants), 1★ reviews 582 → about 20, "fed up with lines" departures about 600 → 5, and about 945 visitors instead of 1,350 (less churn through the lot), net about $65k instead of $79k. Holidays now rate about as ordinary days do. One lift's holidays dropped to 550–580 visitors.

Played through after (two seeds):

| Day | Built after it | Note |
|---|---|---|
| 0 | stages 1 and 2 | $56k left |
| 5–6 | stage 3 | |
| 6–9 | | the guest goal (a holiday with two lifts) |
| 13–14 | stage 4 | an 8-day wait |
| 19 | stage 5 | the 3-day rating 2.73–2.75★ until then |
| 22–24 | | won: the 3-day rating 3.00–3.02★ |

**Wider margins** (the user, 2026-10-12: the 8-day wait for stage 4 should be more like 5). Every price and what guests will pay went up by half together, so guests behave as before and revenue rises by half against unchanged costs: day ticket $60 → $90 (and the price guests think fair), season pass $150 → $225, meal $18 → $27, drink $8 → $12, rentals $40 → $60, a heli ride $10 → $15, the parking reference $20 → $30, guest budgets 90 + 150 × skill → 135 + 225 × skill, and the value-for-money reference budget 150 → 225. The three scenario saves' ticket, pass and lodge prices scaled too. The full resort's ordinary day: revenue about $92k → $135k (tickets $53k, food $35k before), net about $64k → $100k.

Played through after (two seeds, the same days):

| Day | Built after it | Cash after | Note |
|---|---|---|---|
| 0 | stages 1 and 2 | $56k | one lift: 135–210 visitors, $8–20k a day |
| 3 (Christmas) | stage 3 | −$1.37M | |
| 6 | | | the guest goal (MLK Day, two lifts) |
| 8 | stage 4 | −$1.37M | a 5-day wait; the stage-3 resort nets about $97k on an ordinary day |
| 12 | stage 5 | −$1.30M | 4 days |
| 15–16 | | about −$0.9M | won: the 3-day rating 3.00–3.01★ |

Still: the pay and quality system that would let a player trade cost for quality ([[Service Quality]]) isn't built; it's the other lever the user named for the rating.

**Left.**

- The cost-cutting the user wants to make stage 4 a choice (lowering food court quality, the patrol's pay) doesn't exist: service quality isn't the player's to set and doesn't change costs ([[Service Quality]]). Today a player can save only by leaving out line attendants (none are hired here) or closing lifts.
- One lift's holiday crowd meets the guest goal on day 3; fine by the user before (guest targets can move).
- Holidays rate about 2.1–2.3★ at every stage: lift lines and the full lot (about 2,400 turned away).
- The runs are the Goals Test's; the tutorial's own runs will differ.

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
- **The rating goal** is now one day of 1,000+ guests ending at 3.0 stars or better (the user, 2026-10-10; it was a week at 70%, which is 3.8 stars). Ordinary days run 2.5–2.9 and holidays 2.0–2.25 at every stage. Christmas at the full resort: 1,125 of 2,937 went home fed up with lines.
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

So the build order reads as planned: the lot first fills on holidays with the lodge, then every day once Accelerator runs (about 500 turned away on an ordinary day), and the second lot's $150k pays back in about 6 ordinary days. Two lifts and plenty of parking bring the crowds; 49er and the full lodge bring the stars up (2.6 → 2.9 ordinary) and keep guests longer, so fewer churn through on holidays. The goal day is close: the full resort's 1,000+ days ended at 2.77–2.89 stars, against 3.0.

The day-by-day target below predates the 6-day months (Christmas is now day 6 from Dec 1, MLK Day day 9) and needs re-dating with step 7.

## The economics, pinned

Set with the user, 2026-10-10. Costs and running costs are game-wide constants; the starting money is the default for new games and goes into `boreal.save` when it's rebuilt (step 7).

**Starting money**: $700k cash and a $1.5M credit line at 12% a year, accrued per game day (about $500 a day with the line fully drawn), billed at month end. Prices unchanged: day ticket $60, season pass $150, parking free, a meal $18, a drink $8.

**The build order and what each step costs:**

| Step | What | Cost |
|---|---|---|
| 0 | the starting lot (about 200 stalls) | given |
| 1 | California Cruiser (fixed quad, 268 m) and a ticket shed | $626k |
| 2 | cat garage and one cat | $135k (garage $55k for 12 tiles; a 4-tile one is about $20k) |
| 3 | patrol tile and a snowmobile | $39k |
| 4 | small lodge: 6 food court, 2 lounge tiles | $150k |
| 5 | Accelerator (high-speed quad, 770 m) | $1.30M |
| 6 | second cat and a bigger garage | about $100k |
| 7 | second lot | $150k |
| 8 | 49er (double, 708 m) | $506k |
| 9 | the rest of the lodge (10 food court, 6 lounge tiles) | $232k |
| 10 | third cat, second snowmobile | $95k |
| | **Total** | **$3.47M** |

**Build costs** behind it: lift station fees double $400k, fixed triple $475k, fixed quad $550k, high-speed quad $1.15M, 6-pack $1.7M, gondola $4M, plus $150–250 a metre (gondola $2,400); a cat $80k; a snowmobile $15k; a lot $150k; a lodge's first tile brings $30k of foundations, then food court, bar and rental tiles $16k, lounge $12k, tickets $25k, patrol $40k, garage $15k (sheds a quarter of that, tents 0.4).

**Running costs** (per open day; all tripled 2026-10-10, toward [[Vision]]'s 30–50% of revenue): each lift worker $750 (two operators a lift, plus any line attendant); a lift's own running cost $900 (double) to $2,400 (high-speed quad), gondola $7,500, heli $9,000; a cat grooming $3,600 a night, $450 parked; a lodge $600 a day plus per tile: food court and bar $270, rentals $360, tickets $450, patrol $750 (the patroller), lounge $120, garage $90 (sheds half the per-tile rate and $180 base, tents 0.8 and $360); a snow gun $1,200. Closed days: $300 a lift, $120 a building, $450 a cat.

**What each stage earns** (measured, 200-stall first lot, six days from Dec 4):

| After step | Built so far | Open-day costs | Ordinary day: visitors, net | Christmas / MLK: net | Costs as share of ordinary revenue |
|---|---|---|---|---|---|
| 1 | $776k | $3.3k | 221–482, about $16k | $43k / $23k | 15% |
| 2 | $911k | $7.7k | 182–458, about $11k | $39k / $29k | 34% |
| 3 | $950k | $8.3k | 212–460, about $12k | $42k / $26k | 34% |
| 4 | $1.10M | $10.7k | 285–432, about $23k | $58k / $47k | 31% |
| 5 | $2.40M | $14.6k | 493–691 (lot full), about $44k | $50k / $38k | 25% |
| 6 | $2.48M | $18.2k | 499–626 (lot full), about $36k | $42k / $31k | 33% |
| 7 | $2.63M | $18.2k | 710–1,037, about $64k | $143k / $123k | 22% |
| 8+ | $3.47M | $27.6k | 880–1,175, about $83k | $143k / $115k | 25% |

Steps 2, 3 and 6 lower the day's net: grooming and patrol don't pay off yet (change 4).

**Day by day**, a player buying each step as soon as cash and credit allow (from the measured nets; plain holidays estimated between ordinary and named):

| Day | Date | Builds | Cash after |
|---|---|---|---|
| 1 | Dec 1 | Cruiser, tickets, garage and cat, patrol, small lodge | −$250k |
| 4 | Dec 4 | Accelerator | −$1.47M |
| 6 | Dec 6 (Christmas) | second cat | −$1.49M |
| 10 | Jan 4 | second lot | −$1.49M |
| 17 | Feb 5 | 49er: three lifts open | −$1.46M |
| 20–21 | Mar 2–3 | the rest of the lodge, third cat, second snowmobile | −$1.47M |

So three lifts open on about day 17, and everything's built by day 21, a little past the 20-day target. The goal day (1,000+ guests at 3.0 stars) comes with the three lifts: the full resort's ordinary days drew 880–1,175 at 2.77–2.96 stars. A cautious player who keeps $100k of credit spare gets three lifts on day 20 and finishes on day 24. Waits between purchases: 3 days, 2, 4, then 7 before 49er, the big project. The credit line is nearly all drawn at the end, which the season after pays off.

## The target (before pinning)

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
5. **Prices**: day ticket $75 (from $60); food spend about $35 a visitor, about today's. Not done: prices stay at $60 for now (The economics, pinned).
6. Done 2026-10-10 (The economics, pinned): **Costs**, toward the target table:
   - Cruiser about $600k (today $754k; the fixed quad's $700k station fee is most of it)
   - a 4-tile garage and a cat about $100k (cat $80k, today $150k)
   - small lodge about $150k (today $270k for 6 food and 2 lounge tiles)
   - Accelerator about $1.3M (today $1.65M)
   - patrol ($40k), the second lot ($150k) and 49er ($506k) are about right
7. Partly done 2026-10-10: running costs tripled, to 15–34% of revenue by stage. **Step 5 needs something to check.** Today the only staffing choice is a lift's line attendant, and running costs are a few percent of revenue. Either the lodge's staffing comes first ([[Service Quality]]: staff pools per room), or step 5 is prices and attendants only, with running costs nearer [[Vision]]'s 30–50%.
8. Done 2026-10-10 as the defaults: **Starting money**: $700k cash and a $1.5M credit line at 12% (the editor's Money tab, [[Scenario Editor]]). Interest on $1.5M is about $500 a day: small, but the loan should show on the books.

## Real time

Normal speed is now a game day in 2,560 s (43 minutes, [[Season Calendar]]), so the tutorial can't be played at normal: 20 days would take 14 hours. At fast (160 s a day) the 20 days take about 55 minutes, plus building time. So the tutorial should ask the player to watch the first morning at normal and then fast-forward, and the gap between normal and fast (16 times) may need a speed between them.

## Steps

1. Done: the season length (change 2): 6-day months, demand decoupled.
2. Done: demand grows with what's built (change 1): terrain served by skill and run count, with the rating (Measured after steps 1–2). Left: why stage 4 draws less than stage 1.
3. Done: turned-away guests counted and shown (change 3); the starting lot's size is set in step 7. Also done: the stage 4 dip (occupancy counted the lodge against terrain capacity).
4. Grooming and patrol effects (change 4).
5. Done: costs, running costs and starting money (changes 6, 7 in part, 8); prices left at $60.
6. Step 5's content (change 7).
7. Rebuild `boreal.save` as the start: one lot, $700k, the credit line, the goals.
8. Play it through at fast speed, and re-measure the stages against the target table.

## Measuring

`go run ./tools/tutorial assets/scenarios/boreal.save "assets/scenarios/Boreal (Goals Test).save" -credit` plays the tutorial through (Played through, above); `-cash`, `-limit`, `-lodge food,lounge`, `-cats` and `-seed` try other starts. A 20-day run takes about 5 minutes.


`go run ./tools/stages "assets/scenarios/Boreal (Goals Test).save" <stage> <days>` builds a stage from the Goals Test save by removing what comes later, prices it, and runs it headless; it's how the table above was measured, and how each change should be re-measured.

## Open questions

- Does the tutorial let the player build out of order, or guide each step (grants, as in [[Economy Balance]])?
- Which dates should the tutorial open on: Dec 1, so Christmas is day 6 and MLK Day day 9?
- Settled (the user, 2026-10-10): no magic carpet in the demo; guest targets can move, but the rating goal (now a 1,000+ day at 3.0 stars) and three lifts stay, and it's fine if two lifts would meet the guest goal.

## Left over

Moved from [[Next Steps]] on 2026-10-14, when it was cut down to new features.

- [[Boreal Tutorial]]: the tutorial's build order and day-by-day money; first, guests have to follow what's built, and the season's length decided.

## Log

- 2026-10-10: Planned with the user: the build order. Each stage measured from the Goals Test save; guests barely follow what's built, and grooming, patrol and the second lot change nothing measurable. Target day-by-day timeline and the changes it needs.
- 2026-10-10: Steps 1–2 (the user): 6-day months; demand decoupled from the season (`dailyVisitShare` 0.1) and drawn by the terrain the running lifts serve, by skill and run count (`terrainDraw`). Re-measured: Accelerator triples ordinary days; stage 4 draws less than stage 1, undiagnosed.
- 2026-10-10: The stage 4 dip diagnosed and fixed (occupancy counted lodge guests against terrain capacity); full lots counted in the day's stats and report; re-measured. The user: no magic carpet in the demo, guest targets can move, the rating goal and three lifts stay.
- 2026-10-10: Measured with a 200-stall first lot: it fills on holidays with the lodge and every day with Accelerator; the second lot pays back in about 6 days. `tools/stages` gained `-hourly` and `-stalls`.
- 2026-10-10: The goal day (the user): 1,000+ guests ending at 3.0 stars or better, replacing a week at 70%.
- 2026-10-10: Economics pinned (the user): build costs cut (fixed quad and high-speed quad station fees, cats $80k, lodge tiles), running costs tripled, $700k cash and a $1.5M credit line by default; re-measured all eight stages; three lifts by about day 17, everything by day 21.
- 2026-10-12: Played through headless (`tools/tutorial`) with the user's four stages: stage 3 waited 21 days and 3★ was out of reach. Changed: the credit line to $1.4M, and highlights offset annoyances in reviews. Now stage 3 on day 3, stage 4 on day 11, the lodge and cats grown on day 17, won on day 19.
- 2026-10-12: Demand retuned (the user): `terrainDrawCells` 1,000 → 3,000, `terrainDrawTrails` 2 → 6, `dailyVisitShare` 0.1 → 0.2. One lift: 100–210 a day, about 700 on holidays. Stage 3 on day 6, stage 4 on day 14–15, won on day 16–22.
- 2026-10-12: The 3-day rating (the user) made the tutorial unwinnable (every 3 days hold a holiday at 2.2–2.4★); measured what a player can do about holidays; then guests who haven't ridden yet join lines of up to 90 minutes (from 30). Holidays now rate like ordinary days; won on day 22–24.
- 2026-10-12: Prices and guest budgets up by half together (the user: a 5-day wait for stage 4, not 8): stage 3 on day 3, stage 4 on day 8, stage 5 on day 12, won on day 15–16.

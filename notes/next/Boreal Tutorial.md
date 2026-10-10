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

1. **Guests follow what's built.** How many want to come should grow with the lifts and terrain (about 450 for one beginner lift, 800 with Accelerator, 1,000 with all three; today 280 to 420). This is [[First Week Balance]] step 2, and the biggest change: without it nothing after the first lift pays back.
2. **The season's length is a demand lever** (the user's hunch, and it holds): a guest's visits a season are spread over `world.SeasonDays`, so the 50-day season sets how many come each day. A 30-day season gives every day about 1.7 times today's crowd, which by itself closes most of the gap for the first lift. It can't make the crowd grow with what's built (change 1), and it shortens every scenario's calendar too. Decide this with the user before tuning anything else.
3. **Lots fill.** A lot should take about one car a group for the day, so the starting lot (about 200 stalls) holds about 500 guests. Arrivals past that are turned away with "Lots full", which already logs; the day report should say how many. That's what makes the second lot worth building, and what Christmas teaches.
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

1. Decide the season length with the user (change 2).
2. Make demand grow with what's built (change 1), re-measuring the stages each time.
3. Lot capacity and turned-away guests (change 3).
4. Grooming and patrol effects (change 4).
5. Prices, costs and starting money (changes 5, 6 and 8).
6. Step 5's content (change 7).
7. Rebuild `boreal.save` as the start: one lot, $700k, the credit line, the goals.
8. Play it through at fast speed, and re-measure the stages against the target table.

## Measuring

`go run ./tools/stages "assets/scenarios/Boreal (Goals Test).save" <stage> <days>` builds a stage from the Goals Test save by removing what comes later, prices it, and runs it headless; it's how the table above was measured, and how each change should be re-measured.

## Open questions

- Season length (change 2).
- Does the tutorial let the player build out of order, or guide each step (grants, as in [[Economy Balance]])?
- Which dates should the tutorial open on: Dec 1, so Christmas is day 10?

## Log

- 2026-10-10: Planned with the user: the build order. Each stage measured from the Goals Test save; guests barely follow what's built, and grooming, patrol and the second lot change nothing measurable. Target day-by-day timeline and the changes it needs.

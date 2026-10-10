---
title: Economy Balance
kind: plan
status: planned
---

# Economy Balance

Pace money so the player is never long without something worth buying, and never has everything. Targets are set in real minutes, because that's what the player waits through; the calendar ([[Season Calendar]]) then decides how many game days that is. Brings together [[Vision]]'s reference economy (section 7), [[First Week Balance]], and the "Full cost rebalance" in [[Next Steps]]; touches [[Finance]], [[Demand]], [[Scenarios]], and [[Scenario Goals and Rules]].

## Why

From the user, 2026-10-10: there's a fine line between making the player waste time fast-forwarding and just giving them all the money they need. RollerCoaster Tycoon's scenarios and Parkitect are well balanced. Waiting 1–3 game days for cash for the next project, or to pay off a loan to make room for the next one, feels right.

## Pacing targets

At 80 s a game day at the fastest speed ([[Season Calendar]]):

- **Something small every 1–3 days** (1.5–4 minutes at the fastest speed): a snow gun or two, another cat, a food or lounge module, glading a run, more parking, a parcel, or paying down the credit line.
- **A big project every week or two** (a lift, a lodge, a lift upgrade): about 3–5 a season.
- **Prestige projects across seasons**: a gondola, a second base area. [[Vision]] already has the gondola as a multi-season savings goal.
- **Never more than about 5 minutes with nothing worth doing.**

The gap between big projects grows as the resort does, because later lifts are longer and higher, upgrades cost more than the lift they replace, and the gondola costs 6–10 times a quad. Small purchases fill the gaps.

## The measure: payback days

For each thing the player can build: its cost ÷ the extra net income a day it brings (more guests, a better rating, more spending).

- Payback of a few days for everything: money snowballs, and the player soon has all they want (Cities: Skylines after the first hour).
- Payback of 60 days for everything: the player fast-forwards.
- Aim for about 5–15 days early on, getting longer later, so late spending shifts toward the rating, the goals, and prestige rather than profit.

## What the good examples do

- **RollerCoaster Tycoon**: tight at the start, with growth paid for by a loan with interest; each scenario has a goal and a deadline, so waiting has a cost; money piles up late, but the deadline is the pressure by then.
- **Parkitect**: scenarios vary the money (a tight budget, a "make $X" goal); running costs grow with the park (staff, upkeep, ageing rides), so a big park isn't a cash fountain; contracts pay bursts for doing particular things.
- **Prison Architect**: grants pay out for building what the game wants built next, so money and teaching are one thing. A fit for [[Boreal]], the tutorial.
- **Two Point Hospital**: star goals set the pace; loans and rising costs stop coasting.
- **Cities: Skylines**: the cautionary tale, tight for an hour and swimming in money after, because income grows faster than costs.

## The season's own pacing

Real ski resorts pace money already:

- Season passes sell before opening: a lump of cash to build with.
- Christmas and the February holidays are paydays; January's lull and bad-snow weeks drain.
- The off-season is the natural time to skip forward, and the natural build season.

## Scenario length and content

With a 50-day season, 1–3 days per purchase is 16–50 purchases a season, and running out of content by year 2 is a risk (the user, 2026-10-10). Splitting small purchases from big projects (above) is most of the answer. The rest is how long a scenario is meant to last:

- Decide each scenario's length in real hours, then the calendar follows. Later scenarios might run 2–4 seasons.
- **The tutorial is 30–60 minutes** (the user, 2026-10-10), like other games' first scenarios (RollerCoaster Tycoon's Forest Frontiers, Two Point Hospital's Hogsport at one star; from memory, not checked). A learning player spends much of it at normal speed or paused: at about half and half, a game day takes about 160 s, so 30–60 minutes is about 11–22 game days (22–45 at the fastest speed only). [[Boreal]]'s required goals should be reachable in about 2–3 game weeks, December into early January: Christmas on day 10 is the day for its guest goal, the 7-day rating streak runs through late December, and "within season one" becomes a loose bonus. Getting from the minimum resort to three lifts in about 20 days means a lift every 7–10 days, the quick end of the big-project pace, which a tutorial grant or pre-season pass sales could cover.
- A scenario should end about when its terrain runs out of good lift lines. Past that, sandbox play needs money sinks rather than content: ageing lifts and their upkeep, staff costs that grow, guests who expect more each season.
- If scenarios want more seasons than the content holds, the calendar (days a month, months a season) is the lever, as the user suggested.

## Measured

2026-10-10, working backwards from the Boreal Goals Test save, a finished three-lift resort that should meet [[Boreal]]'s goals. Tools: `go run ./tools/econ <save> [days]` prices a save and runs it headless, printing each day's money; `go run ./tools/days <save> [days]` reports each day's visitors, stars, falls and rides. (A one-off that matched a save's lifts to the OpenStreetMap ones is gone.)

**What it costs: $4.49M**, before roads, parcels and glading. The three lifts are real Boreal lifts, all from the base, and the build order is plain:

| Stage | What | Cost |
|---|---|---|
| A: open | California Cruiser (fixed quad, 268 m, 38 m vertical, the beginner lift) | $754k |
| | parking, tickets, cat garage and one cat, patrol shed and one snowmobile, about a third of the lodge | about $650k |
| B: main lift | Accelerator (high-speed quad, 770 m, 161 m vertical), a second cat | $1.80M |
| C: finish | 49er (double, 708 m, 143 m vertical) | $506k |
| | the rest of the lodge (16 food court and 8 lounge tiles in all), a third cat, a second lot, a second snowmobile | about $780k |

Lifts are $2.9M of it. The fixed quad's $700k station fee makes the short beginner lift dearer than the 708 m 49er.

**What it earns**, 14 days headless from Dec 7 with everything open:

| | Visitors | Revenue | Costs | Net |
|---|---|---|---|---|
| Ordinary day | 400–540 | $41–55k | $9.2k | $32–46k |
| Holiday (Christmas, MLK Day) | about 1,000 | $100–105k | $9.2k | $90–96k |
| Average | | | | $46.6k |

About $100 a visitor: $57 in tickets and passes, $40 in food. Running costs are 17% of revenue, under [[Vision]]'s 30–50%. No interest: cash stayed positive.

**Against the tutorial.** With $1M cash and a $1M credit line, the resort has to earn $2.49M itself: 53 days at the finished resort's average, more like 80–100 with a smaller resort earning less early on, against about 20 (Scenario length and content, above). Stage by stage: stage A on day 0 (about $1.4M); Accelerator by about day 6, so it runs on Christmas (day 10), needs about $1.2M from a beginner-only resort; stage C (about $1.28M) from day 6 to 20 needs about $85k a day net, twice today's full resort. Not measured: what a beginner-only resort earns, and how fast a player builds.

**Levers to close it**, best mixed:

1. **Cheaper lifts**: halving station fees and making short beginner lifts cheap takes the total to about $3.1M.
2. **Prices**: $60 is low for Boreal; $90 might raise ticket revenue 40–50% after the guests it loses.
3. **More guests**: lifts run at 12–25% of capacity on an ordinary day; a pool that grows with what's built ([[First Week Balance]] step 2) might bring 800 or more.
4. **Tutorial money** (the editor's Money tab): $2M cash and a $1M line buys stages A and B on day 0, and Christmas pays toward 49er; stage C then takes about 30 days at today's income, or 15 with lever 1 or 2.

Next, when balancing resumes: run headless days with only stage A built, then A and B, for the early income. Done 2026-10-10, stage by stage, in [[Boreal Tutorial]]: guests barely follow what's built.

## Steps

1. **Season economy report.** A headless run of a whole season (building on `tools/days` and `tools/econ`) printing revenue and costs by category, per guest and per day, cash over time, and payback days for each thing built.
2. **Measure three reference builds** of Boreal: the minimum resort, the three-lift resort of its goals, and a big one.
3. **Set scenario lengths** in real hours with the user, and with them the calendar, if it has to change.
4. **Tune, one lever at a time**, re-running the report each time:
   1. running costs that grow with size (lift upkeep, staff, grooming hours): the main guard against late-game money piles;
   2. build costs against daily income;
   3. starting money and credit (the editor's Money tab, [[Scenario Editor]]);
   4. prices and how guests respond to them;
   5. the guest pool, once it scales with what's built ([[First Week Balance]] step 2).
5. **Goal-tied money**, later: grants in the tutorial, and one-off events (a race sponsorship, a film shoot) that top up the early game without raising everyone's base income.
6. **Show money coming**: "you can afford this in about 3 days", with the flashes in [[Money Feedback]]. Players fast-forward less when they can see whether waiting will help.

[[First Week Balance]] becomes the opening slice of steps 2 and 4.

## Open questions

- How long should each scenario after the tutorial be, in real hours? This decides whether the calendar changes.
- Do upgrades (double to quad to high-speed) and expansions count as the content that fills later seasons, or does the game need more kinds of things to build?
- Which money sinks for the sandbox: ageing and upkeep, staff wages, rising guest expectations?

## Log

- 2026-10-10: Planned with the user: pacing targets in real minutes (1–3 days for the next purchase), payback days as the measure, lessons from other games, and the worry that a 50-day season holds more purchases than the game has content for.
- 2026-10-10: The tutorial is 30–60 minutes (the user): Boreal's required goals in about 2–3 game weeks.
- 2026-10-10: Measured backwards from the Boreal Goals Test save: $4.49M to build, $46.6k a day net; the tutorial needs about 4× today's income against costs. Balancing paused for feature work (the user).
- 2026-10-10: The day and economy tools are kept in the repo as `tools/days` and `tools/econ`; the other one-off measuring tools were deleted.
- 2026-10-10: [[Boreal Tutorial]]: the tutorial's build order and day-by-day money, each stage measured; the season's length is a demand lever (visits a season over `SeasonDays`).

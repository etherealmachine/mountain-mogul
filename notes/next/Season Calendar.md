---
title: Season Calendar
kind: plan
status: partial
---

# Season Calendar

A game calendar of our own, shorter than the real one, so a season fits the time other management games give a year: 10 days a month, open December to April, a day in 80 seconds at the fastest speed and in 320 seconds at normal. Changes [[Calendar]]; bounded by [[Crowd Scale]] and [[Fast-Forward Performance]]; reshapes [[Demand]] and [[Finance]].

## Why

From the user, 2026-10-09: the game has two modes of play, and every game in its genre has the same pattern.

- **Paused or normal speed**: building, managing staff, and responding to events in real time (an avalanche: moving resources around, dispatching patrol).
- **Fast forward**, for three reasons:
  - **boredom**: overnight, or operations that are running fine;
  - **data collection**: run a day to watch where the problem points are;
  - **money collection**: earn enough to pay off a debt or save for the next project.

Simulation time, the number of agents, real play time, and the economics have to be balanced together. Today they aren't. A 186-day season (`seasonDaysApprox`) takes about 18 hours of play even when watched at the top non-turbo speed, and the turbo speeds don't help while skiers are out (Measured, below). The other games finish their money cycle in under an hour (Comparisons).

## Targets

From the user, 2026-10-09:

- **10 days a month**, open **December through April**: a 50-day season.
- **80 seconds a day at the fastest speed**, which sets the season at about 67 minutes.
- **Normal speed is a quarter of that**: 320 seconds (5.3 minutes) a day.

Speeds are written as real time per game day, not as multipliers, because what a multiplier would multiply isn't decided yet (How much a game hour holds, below).

| | Fastest | Normal |
|---|---|---|
| Day | 80 s | 320 s |
| Month (10 days) | 13.3 min | 53 min |
| Season (50 days) | 67 min | 4.4 h |

**Inside a day**, assuming the night is skipped and the clock runs evenly from 6:00 to 17:00:

| | Fastest | Normal |
|---|---|---|
| Night (17:00–6:00) | about 2 s | about 2 s |
| A game hour, 6:00–17:00 | 7.1 s | 29 s |
| Arrivals (6:00–9:00) | 21 s | 87 s |
| Open hours (9:00–16:00) | 50 s | 203 s |
| Closing (16:00–17:00) | 7 s | 29 s |

At an even pace through the night, a game hour would be 3.3 s at the fastest speed and most of the day would be an empty mountain.

## How much a game hour holds

Today a game hour holds 900 seconds of skier movement (`world.SimSecondsPerHour`). Movement runs at real speeds, so the clock already runs four times faster than motion ([[Calendar]]). The fastest speed has to play a game hour's movement in 7.1 seconds. That rate sets how many skiers it can carry:

| Game hour holds | Movement per real second (fastest / normal) | Skiers the fastest speed carries (headless) | A 2-minute run at normal speed | Runs a guest gets, against today |
|---|---|---|---|---|
| 900 s (today) | 127 / 31 | about 85: already short | 4 s | all |
| 600 s | 85 / 21 | about 130 | 6 s | ⅔ |
| 450 s | 63 / 16 | about 175 | 8 s | ½ |
| 300 s | 42 / 10 | about 260 | 12 s | ⅓ |
| 225 s | 32 / 8 | about 340 | 16 s | ¼ |

The skier counts are scaled from one measurement (about 110 seconds of movement a real second with about 100 skiers, no rendering), assuming cost grows in line with skiers. Rendering lowers them.

- **Normal speed shows flow, not skiing.** Even at 225 s an hour, a run goes by in 16 seconds: readable, like Parkitect guests on a path, but not watching a turn. Close watching would need its own slow view, such as the follow camera.
- **Fewer runs a day was tried and rejected once.** A game hour held 180 s until 2026-10-07. That left guests two or three runs a day, a 324 m walk from the car taking 2.6 game hours, and lifts carrying a fifth of what they should ([[Calendar]]). 450 s would keep about half of today's dozen or more.
- **Hour-based tuning moves with it**: needs (hunger, thirst, patience), the moments grace period (a quarter hour, [[Scoreless Rating]]), meal and rest lengths, and lift capacity per game hour. Boredom and great-run counts scale with runs, so they mostly carry over.
- **Other ways to make room** don't change what an hour holds: a longer sim step (a 1/10 s step cost little fidelity in [[Crowd Scale]]), cheaper replanning and walking paths, and the sim on its own thread ([[Fast-Forward Performance]]). The user wants one simpler sim for every guest, not coarse off-screen guests ([[Crowd Scale]]).

## The calendar

- **Months** of 10 days each. The months the resort can't open (May to November) pass quickly: the snowpack, construction and finances still run, but there are no guests to simulate.
- **Two holidays a month**, on the 5th and the 10th: the busy days, standing in for weekends and real holidays. Each rush lasts one day, so the player survives a peak, then has four quiet days to fix what it showed. A 50-day season has 10 of them.
- **Day types**: an ordinary day or a holiday. Demand by day type replaces the per-real-date rate.
- **Four named holidays**, the season's biggest rushes, each on the holiday nearest its real date (a game day is about three real ones):
  - **Christmas**: December 10th (December 25th is late in the month).
  - **MLK Day**: January 5th (the third Monday of January, the 15th to 21st, is mid-month).
  - **Presidents' Day**: February 5th (the third Monday of February, the 15th to 21st).
  - **Easter**: moves with the real year, on the holiday nearest its date (March 22nd to April 25th): March 10th for a late-March Easter, April 5th for most years, April 10th for the latest ones. It's the season's last rush, and late snow can make or break it.
  - The other six holidays are unnamed.
- **Storms and weather** move to the short calendar: storms every few game days, not every few weeks, or the season sees two.
- **The sun's path** can still use the real date: day 5 of January is January 15.

## Economics

- **Per-day costs and income stay per day**, so a 50-day season earns and spends about a quarter of a 186-day one. Prices of buildings, lifts and loans have to come down with it, or guests per day go up ([[Demand]]).
- **Attendance** is bounded by the skiers the fastest speed can carry: at 175–260 at once, a day brings in about 400–600 visits through turnover.
- **Scenario goals** written against a real year (season revenue, visits) are rescaled.

## Comparisons

Measured from public sources on 2026-10-09; sources at the end.

| Game | Speeds | A day at normal speed | A year |
|---|---|---|---|
| RollerCoaster Tycoon 1/2 | one speed originally; fast forward in Classic and OpenRCT2 | 12.8 s (a calendar day, no day and night) | the March–October season, 55 min |
| Parkitect | three | 7.5 min (developer) | ? |
| Planet Coaster | three | 2–3 min (player estimate); skips closed nights | ? |
| Planet Zoo | three | ? | 18 min; Planet Zoo 2 slowed it to 60 |
| Two Point Hospital | slow, normal, fast | 4 s | 27 min |
| Prison Architect | three, the fastest about 5 times normal | 24 min | — |
| RimWorld | three, the fastest 6 times normal | 16.7 min | 60 days: 2.8 h at the fastest |
| Cities: Skylines II | three, the fastest 4 times normal | 72 min (one day-night cycle is a month) | 3.6 h at the fastest |
| Jurassic World Evolution 2 | three; drops to normal during a storm or disaster | — | — |

- Fastest speeds run 2–6 times normal; the target here is 4.
- Disasters drop the speed in JWE2, and turbo already stops before a storm here ([[Calendar]]). An avalanche, injury or lift breakdown can do the same.

## Measured

2026-10-09, headless, on a save of Boreal open on day 2 (about 100 guests at peak), stepping the sim as fast as the CPU allows:

- Night: a game hour in 0.03–0.15 s.
- Arrivals (6:00–9:00, lifts not yet running): 0.1–0.4 s a game hour.
- Open hours (9:00–16:00, 75–115 guests): **6–9.7 s a game hour**, about 110 seconds of movement a real second. The drop comes when skiing starts at 9:00, so it's skiers, not guests or the map.
- A whole day: 59–62 s. Today's turbo speeds run at this ceiling during open hours, whatever is chosen.

The "Boreal (Goals Test)" save starts closed with its lifts stopped, so headless it draws no guests.

## Steps

1. **Measure** the headless ceiling at 450 and 300 s a game hour, with a larger crowd on the day-2 save, to test the "in line with skiers" assumption. Pick what a game hour holds.
2. **Calendar**: 10-day months, the open season, holidays on the 5th and 10th, and the quick off-season.
3. **Speeds** as real time per day: normal and fastest, plus pause; a night skip; dropping to normal on an emergency.
4. **Rescale** hour-based tuning, demand and the economy, and check Boreal's day and season.

## Open questions

- What a game hour holds (step 1).
- Whether the night is skipped or runs fast at every speed.
- What happens to the off-season months: skipped with a summary, or fast with building allowed.
- Whether there's a speed between normal and fastest.
- How a day's place in the week and month is shown to the player.

## Sources

- [RCT ticks, days and months (speedrun.com)](https://www.speedrun.com/de-DE/rct1/forums/6hmli)
- [RCT Classic fast forward (Coaster101)](https://www.coaster101.com/2016/12/31/review-roller-coaster-tycoon-classic/)
- [Parkitect day and night length (Steam, developer reply)](https://steamcommunity.com/app/453090/discussions/1/804595355558791782/)
- [Planet Coaster day length (Steam)](https://steamcommunity.com/app/493340/discussions/0/1692662484255546693)
- [Planet Zoo year length (Steam)](https://steamcommunity.com/app/703080/discussions/0/1735507550908407972)
- [Two Point Hospital time and speeds (Steam)](https://steamcommunity.com/app/535930/discussions/0/1737715419894525453)
- [Prison Architect Clock (fandom wiki)](https://prison-architect.fandom.com/wiki/Clock)
- [RimWorld Ticks](https://rimworldwiki.com/wiki/Ticks)
- [Cities: Skylines II time scales (Steamah)](https://steamah.com/cities-skylines-ii-time-scales-guide/)
- [Jurassic World Evolution 2 accessibility](https://www.jurassicworldevolution.com/en-US/2/accessibility)

## Log

- 2026-10-09: Comparisons gathered; a day of Boreal timed headless.
- 2026-10-09: The user: our own calendar with shorter months; open December to April; 10 days a month; 80 s a day at the fastest speed and 320 s at normal. Speeds to be talked about as real time per day until what a multiplier multiplies is decided.
- 2026-10-09: The user: two holidays a month, on the 5th and 10th, so each rush lasts one day.
- 2026-10-09: The user: three named holidays, Christmas, MLK Day, and Presidents' Day.
- 2026-10-09: The user: Easter too, a fourth named holiday.
- 2026-10-09: Built the calendar: 10-day months (`world/calendar.go`); each game day is the first real day of a stretch of about three, and the rollover plays the rest of the stretch's weather and melt (`playOffscreenDays`), so a month gets a month of snow; game dates in the top bar, event feed, charts, day report and editor; holidays on the 5th and 10th, named ones (Christmas Dec 10, MLK Day Jan 5, Presidents' Day Feb 5, Easter by year) shown by the date; seasons close at the end of April. Demand spreads `VisitsPerSeason` over 50 days, by day type (ordinary 0.6, holiday 1.5, named 4.0). Speed buttons are real seconds per day (320, 160, 80), a quiet night or closed day passes in 2 s, and clicking the fastest again skips to the next storm.
- 2026-10-09: Balancing the Boreal Goals Test for Christmas (headless, `cmd/zzseason`): 588 visitors at 2.48★ on the first run. Fixed along the way: season passes left guests with no money for a drink (they now keep a meal and two drinks back); hunger and thirst don't use up the grace period while the guest is on the way to food or drink; the longest line a guest joins is half a clock hour of wait at the lift's real throughput, not 57 people; chairs fill from several loading lanes; lift capacity was counted twice over (Boreal's three lifts carry about 4,100 an hour, not 8,250). Free water added. Christmas now: 631 visitors at 2.74★ with free water, parking full (98 cars turned away); held below 3★ by a too-small food court, no lounge, thirst, and bored novices (a fair complaint with one novice lift). Guests ride about 4 times a day; lifts run at 12–25% of capacity on an ordinary day. A headless ordinary day takes 110–120 s and Christmas 250–290 s, against the 80 s target.
- 2026-10-09: The user's map changes, in a copy of the Goals Test: a second lot (361 stalls), free water, entry pools of 8,000 each, and the food court rebuilt from a 4-tile unheated shed into a heated 6×4 lodge (160 seats, an 80-seat lounge). Also: hunger and thirst press at 0.55 (was 0.4) so a beginner's hour-long descent doesn't overrun the grace period; they drain a quarter as fast off the snow, not at all while being served, and altitude counts for less (the user); the demand cap uses the measured lift cycle (about 1,500 s, three rides a day). Nine days: ordinary days 320–405 visitors at 2.9–3.3★; Christmas 874–884 at 2.86–3.13★ (two runs); MLK Day 898 at 3.09★. With pools of 10,000, Christmas drew 1,007 at 2.99★. Left below 3★: bored novices (fair with one novice lift), falls in holiday crowds, "nothing here is my kind of skiing". A headless Christmas takes about 400 s against the 80 s target.
- 2026-10-09: The user's map changes applied to the Goals Test (pools 10,000 an entry); committed. A first performance pass ([[Crowd Scale]]): a headless Christmas (998 visitors, 3.06★) takes 83 s against the 80 s target, an ordinary day 36–42 s.

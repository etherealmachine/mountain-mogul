---
title: Calendar
kind: system
status: partial
---

# Calendar

Sim time maps to a real date: one calendar day is 24 clock hours of three sim-minutes each.

**Clock time and sim seconds are different units.** Movement runs in sim seconds at real speeds (a skier at 10 m/s, a snowmobile at 12 m/s), but a clock hour is only 180 sim seconds (`world.SimSecondsPerHour`), so the clock runs 20 times faster than motion: a 9-to-4 ski day is 1,260 sim seconds, and in one clock hour a snowmobile covers about 2 km. Anything meant as clock time (an hour's wait, five hours to get hungry, an afternoon's arrivals) has to be written in clock units (`world.SimSecondsPerHour`, `HourOfDay`), not as plain seconds; a duration written as seconds is twenty times longer on the clock than it looks. Speeds stay real, so distances feel about twenty times shorter than the clock suggests. Caught on 2026-10-06 when an injured guest's "one hour" wait was set to 3,600 sim seconds, which is twenty clock hours ([[Patrol Day]]). The year runs continuously with no skipped off-season. Seasons run November through Memorial Day. The sun moves on its real path for the map's latitude (45°N for drawn maps), which drives lighting, shadows, and the melt in [[Snow]]. With a time zone (imported maps) the clock is the place's local time, daylight saving included, so solar noon drifts with longitude and season; without one the clock is solar time.

Opening is the player's choice, made from the popup of any building with [[Tickets]]: the resort is open or closed, and when open, the lifts run during opening hours (9 to 4 by default). Closing mid-day empties the lift lines and sends everyone home. The day rollover advances [[Weather]], charges the day's costs to [[Finance]], writes the daily stats, and opens the day report.

Fast-forward can stop an hour before the next storm. [[Vision]] wants more stop conditions: first freezing night, first snowfall, and a base-depth threshold.

Code: `internal/sim/calendar.go`, `resort.go`, `sun.go`, `storm.go`.

## Log

- 2026-10-01: Year-round calendar, opening hours, the open/close switch, sun path, and skip-to-storm are in. Other skip conditions are not.
- 2026-10-06: Spelled out the clock-versus-sim-seconds pitfall after an "hour" was written as 3,600 sim seconds ([[Patrol Day]]).

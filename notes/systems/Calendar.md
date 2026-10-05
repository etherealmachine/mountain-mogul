---
title: Calendar
kind: system
status: partial
---

# Calendar

Sim time maps to a real date: one calendar day is 24 clock hours of three sim-minutes each. The year runs continuously with no skipped off-season. Seasons run November through Memorial Day. The sun moves on its real path for the map's latitude (45°N for drawn maps), which drives lighting, shadows, and the melt in [[Snow]]. With a time zone (imported maps) the clock is the place's local time, daylight saving included, so solar noon drifts with longitude and season; without one the clock is solar time.

Opening is the player's choice, made from the popup of any building with [[Tickets]]: the resort is open or closed, and when open, the lifts run during opening hours (9 to 4 by default). Closing mid-day empties the lift lines and sends everyone home. The day rollover advances [[Weather]], charges the day's costs to [[Finance]], writes the daily stats, and opens the day report.

Fast-forward can stop an hour before the next storm. [[Vision]] wants more stop conditions: first freezing night, first snowfall, and a base-depth threshold.

Code: `internal/sim/calendar.go`, `resort.go`, `sun.go`, `storm.go`.

## Log

- 2026-10-01: Year-round calendar, opening hours, the open/close switch, sun path, and skip-to-storm are in. Other skip conditions are not.

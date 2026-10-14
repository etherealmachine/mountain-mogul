---
title: Night Sessions
kind: plan
status: planned
---

# Night Sessions

Lit runs and lifts and an evening session after dark, for locals after work and day guests who stay on. Builds the [[Night Skiing]] card; touches [[Calendar]], [[Lifts]], [[Trails]], [[Demand]], [[Tickets]], [[Grooming]], [[Finance]], and [[Rendering]]. [[Snowmaking on Runs]] comes first: a man-made base keeps lit runs open in a thin season. *For* [[Boreal]], which really does run at night.

## What's there today

Surveyed 2026-10-14:

- **Hours** are resort-wide (`World.OpenHour`, `CloseHour`, 9 to 16 by default), set in the ticket window's popup up to 23:00. Nothing checks for dark: lifts run to the close whatever the sun.
- **The sun and moon** follow the real date and latitude (`sim/sun.go`), so a December dusk is about 16:30 at 45°N. Moonlight takes over below −3°.
- **Grooming** starts at midnight and runs to half an hour before opening, so a late close doesn't shorten it; guests leave up to about 47 clock minutes after closing.
- **Arrivals** are all tied to opening (`RollArrivalOffset`, up to four hours after it); nothing brings guests in the evening.
- **Costs** are per open day, not per hour, so a long day costs the same as a short one.
- **Lights**: the shaders take 16 spotlights (`MAX_LAMPS`), used only by the cats, on the terrain and static shaders; figures, cars and lifts aren't lamp-lit. Nothing on buildings lights up.

## Decisions

From the user, 2026-10-14:

- **Light a run or a lift** from its popup, by the metre to install and by the hour to run; poles go along it on their own.

## How it works

- **Day and night sessions.** The ticket window keeps the day's hours and adds a night close (off, or 18:00 to 22:00). After dusk (sunset plus 20 minutes) only lit lifts load, and guests plan only lit runs; at the night close everything shuts as today.
- **Lights.** A run's popup offers lights (about $120 a metre); a lift's lights its line and stations (about $60 a metre). Lit runs and lifts are `Trail.Lit` and `Lift.Lit`, saved. Lights come on at sunset and go off at the night close; each lit hour costs by length (about $0.50 a metre an hour).
- **Who comes.** A night ticket (on sale from 16:00, half price by default) and evening guests: locals arriving 16:30 to 19:00 for two or three hours, more with more lit terrain. Day guests with energy left stay on if their runs are lit.
- **Costs by the hour.** A lift's running cost and staff are charged per open hour (today's day rates over a 9-to-4 day), so a late night costs what it should.
- **Grooming.** Lit runs are groomed after the night close; the cats' midnight start already allows for it. A cat waits for a run to empty before it starts on it.
- **Drawing it.** Poles along lit runs (a light pole model), their heads glowing. Their light goes into a light map over the terrain (a world-space texture each pole adds a pool to) that the terrain, static and dynamic shaders sample after dark, so any number of poles costs the same; figures, chairs and cars on a lit run are lit too. Lodges' windows glow at night, and lit lift stations get lamps.

## Steps

1. **Sessions and costs**: the night close and dusk rule; guests planning only lit runs and lifts after dark; costs per open hour.
2. **Lights**: lighting runs and lifts in their popups, the cost, poles placed along them.
3. **The light map**: lit snow and figures after dark, glowing pole heads, lit lodge windows and station lamps.
4. **Evening guests**: the night ticket, evening arrivals, day guests staying on.
5. **Grooming and balance**: cats after the night close, waiting for empty runs; Boreal's night session checked headless and on screen.

## Open questions

- How the player sees what's lit by day (a Lights map view, or poles alone).
- Whether night guests rate differently (a highlight for skiing under the lights, a letdown for a dark run they wanted).

## Log

- 2026-10-14: Surveyed and planned with the user, ranked second, after [[Snowmaking on Runs]].

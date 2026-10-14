---
title: Night Sessions
kind: plan
status: partial
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
- **Snowmaking first** ([[Snowmaking on Runs]]). Unlit runs with snowmaking make snow through a night session.

## How it works

- **Day and night sessions.** One closing time, the ticket window's "Lifts close" (up to 23:00), is the night's close: there's no separate night close. After dusk (sunset plus 20 minutes) only lit lifts load (`Lift.InService`), their lines' unlit neighbours send theirs to find one, and guests pick lit runs (an unlit one costs them six times as much to plan, so they still get down); once no lit lift is running the mountain is closed for the day.
- **Lights.** A run's popup offers lights (about $120 a metre); a lift's lights its line and stations (about $60 a metre). Lit runs and lifts are `Trail.Lit` and `Lift.Lit`, saved. Lights come on at sunset and go off at closing; each lit hour costs $0.15 a lit metre, as the day's "Lights" cost line. Not in the demo (its lifts still stop at dusk).
- **Who comes.** A night ticket (from 16:00, half price, on by default; offered while there's a night session: a lit lift, closing an hour or more past dusk) and evening guests: arrivals from 16:30 to 19:00 (or an hour before closing), up to 35% of a day's visits with half the runs lit or more, arriving with 0.6 energy so they ski two or three hours. Day guests stay on while they have the energy and lit runs.
- **Costs by the hour.** A lift's running cost and staff are charged per open hour (today's day rates over a 9-to-4 day; to dusk for an unlit lift), so a late night costs what it should.
- **Grooming.** Lit runs are groomed after the night close; the cats' midnight start already allows for it. A cat waits for a run to empty before it starts on it.
- **Drawing it.** Poles along lit runs every 30 m, alternating sides, each aimed across the run (`light_pole.py`, `world.LightSpots`), their lenses glowing while the lights are on; lamps on a lit lift's towers and at its load and unload points. Built: Their light goes into a light map over the terrain (a world-space texture each pole adds a pool to) that the terrain, static and dynamic shaders sample after dark, so any number of poles costs the same; figures, chairs and cars on a lit run are lit too. Each pole's pool reaches 40 m (a lift lamp's 24 m), centred a third of that out over the run, baked into a 2.5 m light map (`render/night_lights.go`) that `lighting.glsl`'s `nightLight` adds on the terrain, trees, buildings, figures and chairs, rebuilt only when what's lit changes. Left: lodges' windows glowing at night.

## Steps

1. Done: **Sessions and costs**: the night close and dusk rule; guests planning only lit runs and lifts after dark; costs per open hour.
2. Done: **Lights**: lighting runs and lifts in their popups, the cost, poles placed along them.
3. Done, but for lodge windows: **The light map**: lit snow and figures after dark, glowing pole heads, lit lodge windows and station lamps.
4. Done: **Evening guests**: the night ticket, evening arrivals, day guests staying on.
5. **Grooming and balance**: cats after the night close, waiting for empty runs; Boreal's night session checked headless and on screen.

## Open questions

- How the player sees what's lit by day (a Lights map view, or poles alone).
- Whether night guests rate differently (a highlight for skiing under the lights, a letdown for a dark run they wanted).

## Log

- 2026-10-14: Surveyed and planned with the user, ranked second, after [[Snowmaking on Runs]].
- 2026-10-14: Steps 1–2 built (poles left for step 3). Boreal Goals Test, closing at 21:00 with Lift3 and Sunset Blvd. lit (2 km): dusk at 17:00, then only Lift3 loads (about 44 riding, a line of 70); 411 guests still on the mountain at 21:00 and 6 by 23:00. Lights at $0.50 a metre an hour came to about $1,000 an hour, so cut to $0.15.
- 2026-10-14: Step 3 built (poles, lift lamps, the light map). First pools of 26 m read as separate blobs; widened to 40 m at a lower peak they light the run evenly.
- 2026-10-14: Step 4 built. Boreal Goals Test, closing at 21:00: with Lift3 and its runs lit (21% of the run area) about 60 evening arrivals, revenue from 16:00 about $23k, lights $1,300; with everything lit about 300 arrivals, revenue from 16:00 about $50k, lights $4,500. The mountain empties by about 23:00, before the cats start at midnight, so step 5's grooming needs nothing yet for a 21:00 close; closing at 23:00 still to check.
- 2026-10-14: Good for now (the user); balance is listed under Economy in [[Next Steps]].

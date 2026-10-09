---
title: Weather
kind: system
status: partial
---

# Weather

A daily Markov chain with five states: clear, overcast, light snow, storm, and rain. Each month has its own odds and mean temperature. A day sets the temperature range, cloud cover, and how much snow or rain falls. The chain is seeded by date, so the five-day forecast in the top bar is exactly what will happen.

Each day's weather feeds [[Snow]]: new layers on snow days, surface changes on dry days (rain, cold clear, warm clear, wind), and melt through the hourly temperature curve. A big snowfall or a rain day triggers an [[Avalanche]] check. The hourly temperature also decides whether [[Snowmaking]] can run. [[Calendar]] fast-forward can stop just before the next storm.

Imported maps carry a climate from the real place ([[Terrain Import]]): for each month, the mean temperature and daily range, the share of wet days, the precipitation on a wet day, and cloud cover, plus the prevailing storm wind. It comes from Open-Meteo's reanalysis archive anywhere in the world, with temperature and precipitation replaced by the closest-matching SNOTEL snow station within 30 km in the US (Kirkwood uses Carson Pass, 6 km away). The chain builds its monthly odds and amounts from it: wet days split into rain or snow by the base area's temperature, lapsed from the climate's altitude to the map's; dry days into clear or overcast by cloud cover. The draw weights are corrected for each state's persistence, so the long-run shares match the record. Drawn maps keep the generic cold-mountain profile.

Each weather state has its own daily swing: clear days the widest, storms the narrowest. A climate scales them together so the month's average day-to-night range matches the record. The sim plays the season's weather from 1 September up to the start date before the game begins, so the snow on the ground and the weather that follows are the same season.

Weather does not yet affect [[Demand]] or [[Moments]], and wind direction is fixed per scenario rather than changing daily. A wet day is rain or snow over the whole mountain, chosen at the base, so spring days can rain on the summit too.

Spec: [[Weather Spec]]. Code: `internal/sim/weather.go`, `temperature.go`.

## Log

- 2026-10-01: Chain, forecast, snow effects, hourly temperature, and skip-to-storm are in. Weather-driven demand and a daily wind field are listed as future work.
- 2026-10-05: Per-scenario climate from Open-Meteo and SNOTEL, set at import. Kirkwood over 40 simulated years: monthly mean temperatures within 0.3 °C of the record, precipitation within about 15%.
- 2026-10-07: Clear days no longer run 15–20 °C too warm (Boreal in December: highs p90 8.7 °C, max 14.9 °C over 30 years, was 10.5 and 21.6). The season's weather runs from 1 September to the start date, shared with the starting snow.
- 2026-10-09: Skip-to-storm removed with turbo ([[Calendar]]).

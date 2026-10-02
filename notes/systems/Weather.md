---
title: Weather
kind: system
status: partial
---

# Weather

A daily Markov chain with five states: clear, overcast, light snow, storm, and rain. Each month has its own odds and mean temperature. A day sets the temperature range, cloud cover, and how much snow or rain falls. The chain is seeded by date, so the five-day forecast in the top bar is exactly what will happen.

Each day's weather feeds [[Snow]]: new layers on snow days, surface changes on dry days (rain, cold clear, warm clear, wind), and melt through the hourly temperature curve. A big snowfall or a rain day triggers an [[Avalanche]] check. The hourly temperature also decides whether [[Snowmaking]] can run. [[Calendar]] fast-forward can stop just before the next storm.

Weather does not yet affect [[Demand]] or [[Satisfaction]], and wind direction is fixed per scenario rather than changing daily. Every map shares the same monthly odds; the plan is to tailor them per scenario from real-world climate data ([[Scenario Goals and Rules]]).

Spec: [[Weather Spec]]. Code: `internal/sim/weather.go`, `temperature.go`, `storm.go`.

## Log

- 2026-10-01: Chain, forecast, snow effects, hourly temperature, and skip-to-storm are in. Weather-driven demand and a daily wind field are listed as future work.

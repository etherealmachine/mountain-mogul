# Weather System

## Overview

Weather advances one day at a time via a Markov-chain generator (`sim.Chain`). Each day produces a `DayWeather` value that drives snow accumulation, terrain transitions, melt, and the HUD forecast. Nothing else in the game reads individual weather fields directly — all consumers go through `DayWeather`.

## States

| State | String | Conditions |
|-------|--------|------------|
| `WeatherClear` | "Clear" | Sunny, no precipitation |
| `WeatherOvercast` | "Overcast" | Cloudy, no precipitation |
| `WeatherLightSnow` | "Snow" | Light to moderate snowfall |
| `WeatherHeavySnow` | "Storm" | Heavy snow / blizzard |
| `WeatherRain` | "Rain" | Above-freezing precipitation |

## Daily Weather Values (`DayWeather`)

| Field | Type | Meaning |
|-------|------|---------|
| `State` | `WeatherState` | Canonical condition for the day |
| `TempC` | `float32` | Daily mean temperature in °C |
| `CloudCover` | `float32` | 0–1; used by the sky renderer |
| `AccumSWE` | `float32` | New snow in metres SWE (0 on non-snow days) |
| `RainMM` | `float32` | Rainfall in mm (no terrain effect beyond melt) |

## Markov Chain

`Chain.Advance(t time.Time)` steps to the next calendar day. At each step:

1. **Persist or redraw** — the chain stays in its current state with probability `persistence[state]` (0.60–0.75), otherwise draws a new state from the month's tendency weights.
2. **Temperature** — month mean + state offset + Gaussian noise. Hard-clamped for coherence: snow states cap at −0.5 °C; rain requires ≥ +0.5 °C.
3. **Cloud cover** — `cloudBase[state] + rand × cloudRange[state]`, clamped to [0, 1].
4. **Precipitation** — LightSnow: 0.3–1.2 cm SWE; HeavySnow: 1.0–3.5 cm SWE; Rain: 2–20 mm.

Each step is seeded deterministically from `(currentState, calendarDate)` via `daySeed()`, so calling `Advance` and `Forecast` for the same date always produce identical results.

`Chain.Forecast(from time.Time, n int)` returns a speculative n-day lookahead without mutating the chain. It seeds each step identically to `Advance`, so the forecast matches what actually happens when those days arrive — the HUD 5-day strip is always accurate.

## Monthly Profiles

Twelve profiles (January–December) each carry tendency weights `(Clear, Overcast, LightSnow, HeavySnow, Rain)` and a mean temperature.

- **Jan–Feb**: peak snow season; heavy-snow tendency highest, sub-zero temperatures.
- **Mar**: spring thaw begins; rain probability rises, heavy snow falls off.
- **Apr–May**: shoulder; rain dominant, snow rare, near-freezing temps.
- **Jun–Oct**: off-season; almost no snow, warm temperatures.
- **Nov–Dec**: early season; mixed precipitation, returning cold.

## Per-State Parameters

| State | Persistence | Temp offset (°C) | Temp noise σ | Cloud base | Cloud range |
|-------|-------------|-------------------|--------------|------------|-------------|
| Clear | 0.75 | +4 | 4 | 0.05 | 0.15 |
| Overcast | 0.68 | 0 | 4 | 0.70 | 0.25 |
| LightSnow | 0.65 | −2 | 3 | 0.80 | 0.18 |
| HeavySnow | 0.60 | −5 | 3 | 0.90 | 0.10 |
| Rain | 0.60 | +5 | 3 | 0.75 | 0.20 |

## Terrain Effects

On each day rollover (`simulation.go`), `applyDailyWeather` fires:

### Snow days (`WeatherLightSnow`, `WeatherHeavySnow`)

`pushSnowLayer` adds a new `SnowLayer` on every cell. The new layer's `Kind` is determined by storm temperature:

| Storm TempC | New layer Kind |
|-------------|----------------|
| < −5 °C | Powder |
| −5 to −1 °C | Packed Powder |
| −1 to 0 °C (clamped) | Cement |

If the top layer already has the same Kind as the incoming snow (e.g. powder on powder), accumulation is added to it in-place rather than pushing a new entry.

Grooming is diluted proportionally: 2 cm SWE fully buries any groomed surface. Skier tracks are buried by the same factor.

### Non-precipitation days

A **weather event** is derived from the state:

| Condition | Event |
|-----------|-------|
| `WeatherRain` | Rain |
| `WeatherClear`, TempC < 0 | Cold Clear |
| `WeatherClear`, TempC ≥ 0 | Warm Clear |
| `WeatherClear` or `WeatherOvercast`, wind roll fires | Wind |
| `WeatherOvercast` (no wind) | None |

Wind is a stochastic event that fires on Clear and Overcast days. Probability is temperature-driven: 20 % below −5 °C, 12 % between −5 and 0 °C, 5 % above 0 °C. Wind takes priority over Cold/Warm Clear.

`applyKindTransition` then updates the **top layer's Kind** on every cell per the transition matrix (see SNOW.md).

`applyMelt` follows, removing SWE from the top layer using a lapse-rate-adjusted temperature (colder at altitude). Rain days also contribute additional melt proportional to `RainMM`.

### Buried freeze (all sub-freezing days)

After precipitation or kind-transition effects, `applyBuriedFreeze` runs whenever `TempC < 0`. It checks the layer immediately below the surface on every cell. If that buried layer is a wet or saturated kind, it freezes:

- **SlushCorn → Boilerplate** — liquid-saturated slush solidifies to hard ice.
- **Cement → Crust** — dense wet snow hardens into a breakable glaze.

The threshold is lapse-rate adjusted per cell, so high-elevation cells freeze more aggressively than the base lodge area under the same forecast temperature.

### Traffic decay

`SkierTraffic` on every cell decays by 15 % daily (~4-day half-life), preventing stale traffic from blocking kind transitions indefinitely.

## Rendering

Weather is applied to the renderer via `WeatherOverlay int` (set to `int(sim.WeatherState)` each frame).

**Sky colour** (`skyColor()`): the GL clear colour changes per state.

| State | Sky (R, G, B) |
|-------|---------------|
| Clear | 0.635, 0.682, 0.918 — blue sky |
| Overcast | 0.62, 0.63, 0.68 — flat grey |
| Light snow | 0.72, 0.74, 0.80 — light grey |
| Heavy snow | 0.58, 0.60, 0.64 — dark grey |
| Rain | 0.45, 0.50, 0.58 — blue-grey |

**Precipitation overlay** (`weather.frag`): a full-screen pass after the 3D passes.

- **Clear / Overcast**: no overlay.
- **Light snow**: two independent flake layers (7-cell and 14-cell grids) with gentle horizontal sway.
- **Heavy snow**: three flake layers plus a base white fog veil (0.18 alpha minimum) for a whiteout feel.
- **Rain**: two streak layers plus a subtle grey haze overlay.

## HUD

The top bar shows a 5-day forecast strip (today + 4 ahead). Each slot displays a weather icon and temperature in °F.

Icons are drawn by `ui.DrawWeatherIcon` per `ui.WeatherKind`:

| WeatherKind | Icon | Tint |
|-------------|------|------|
| `WKSunny` | Sun | Warm yellow |
| `WKCloudy` | Cloud-sun | Neutral grey |
| `WKSnow` | Cloud-snow | Cool blue-white |
| `WKStorm` | Two cloud-snow icons side-by-side | Pale blue |
| `WKRain` | Drop | Blue |

`weatherToUI()` in `scene/scenario.go` maps `sim.WeatherState` → `ui.WeatherKind`. The forecast matches actual future weather because `Chain.Forecast` and `Chain.Advance` use the same deterministic seed.

## What Weather Does Not Yet Affect

- **Demand / arrivals** — guest arrival probability uses resort rating, terrain match, and lift occupancy. Bad weather does not suppress arrivals.
- **Guest satisfaction** — no weather-satisfaction coupling.
- **Wind field** — wind direction is a static parameter set at scenario load by the procedural snow generator, not updated daily.

---
title: UI and HUD
kind: engine
status: shipped
---

# UI and HUD

Immediate widgets drawn as batched quads in the renderer's UI pass: windows, buttons, sliders, text input, menu bars, and submenus.

The top bar shows money, the resort rating from [[Demand]], the date and the five-day [[Weather]] forecast, game speed, and settings. The bottom toolbar holds the build tools, with submenus for lifts, buildings, transport, terrain, and [[Amenities]]. The left panel is the [[Event Feed]]. Chart windows plot the daily history. A game opens paused. Each midnight the game pauses and a profit and loss report from [[Finance]] opens, with a button to start the next day; the player can turn off either from the report. Testbeds and `-screenshot` runs don't pause. Building and lift popups carry their own controls: prices, open and closed, lift upgrades.

Money changes give little feedback today: some tools show a cost next to the cursor, but nothing shows money leaving or arriving. [[Money Feedback]] plans that.

Settings are units (imperial or metric), whether the daily report opens, whether the game pauses each night, and anti-aliasing, stored outside the save.

Code: `internal/ui/`, `internal/settings/`, `internal/scene/day_report.go`.

## Log

- 2026-10-01: Top bar, toolbar, event panel, charts, day report, popups, and settings are in.
- 2026-10-11: Games open paused and pause each midnight behind the day report ("Start the next day"; "Pause every night" turns it off) ([[Boreal Playtest]]).

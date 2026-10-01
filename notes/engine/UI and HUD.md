---
title: UI and HUD
kind: engine
status: shipped
---

# UI and HUD

Immediate widgets drawn as batched quads in the renderer's UI pass: windows, buttons, sliders, text input, menu bars, and submenus.

The top bar shows money, the resort rating from [[Demand]], the date and the five-day [[Weather]] forecast, game speed, and settings. The bottom toolbar holds the build tools, with submenus for lifts, buildings, transport, terrain, and [[Amenities]]. The left panel is the [[Event Feed]]. Chart windows plot the daily history. Each midnight a profit and loss report from [[Finance]] opens, unless the player turned that off in settings. Building and lift popups carry their own controls: prices, open and closed, lift upgrades.

Settings are just units (imperial or metric) and whether the daily report opens, stored outside the save.

Code: `internal/ui/`, `internal/settings/`, `internal/scene/day_report.go`.

## Log

- 2026-10-01: Top bar, toolbar, event panel, charts, day report, popups, and settings are in.

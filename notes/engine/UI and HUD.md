---
title: UI and HUD
kind: engine
status: shipped
---

# UI and HUD

Immediate widgets drawn as batched quads in the renderer's UI pass: windows, buttons, sliders, text input, menu bars, and submenus.

The look (2026-10-13, after Parkitect): translucent rounded panels floating over the world with a soft shadow, rounded buttons, round close buttons centred in title strips, and hover tooltips. One place sets it: `internal/ui/theme.go` (`DrawPanel`, `DrawPanelHeader`, `CloseButton`, `DrawTooltip`), over the renderer's rounded shapes (`internal/render/ui_shapes.go`). Dialogs use a nearly opaque fill and dim the world behind them.

The top bar is three pods: money, guests and the rating (stars drawn as a shape, since the font has no ★); the date, clock and five-day [[Weather]] forecast; and game speed and the panel toggles, each with a tooltip. The bottom toolbar is a floating dock of picture tiles with tooltips; its submenus open as a row of captioned picture tiles above it. The pictures are the game's own models, rendered once at startup into textures (`internal/render/thumbnails.go`): a lodge, the ticket office, a car, the lift station, each lift's chair, a ski rack, a tree, a snow gun. Tools with no model keep their icons. The left panel is the [[Event Feed]]; the right is Map views (contour, slope, snow, grooming, avalanche risk, moguls, falls; the editor adds trails, ground, and parcels). The Resort Overview window has guests, arrivals and departures, cash, revenue and costs, Reviews (the heart), and the resort's terrain figures.

The font atlas holds printable ASCII and the few other characters the UI uses (— – → ← ° × · ² … © ≈ ’ é); anything else draws as a gap. Menu titles use a second, 64-point face.

A game opens paused. Each midnight the game pauses and a profit and loss report from [[Finance]] opens, with a button to start the next day; the player can turn off either from the report. Testbeds and `-screenshot` runs don't pause. Building and lift popups carry their own controls: prices, open and closed, lift upgrades.

Money changes give little feedback today: some tools show a cost next to the cursor, but nothing shows money leaving or arriving. [[Money Feedback]] plans that.

Settings, from the start menu or the in-game menu: units (imperial or metric), full screen, anti-aliasing (off, 2x or 4x, applied at once: the 3D scene draws into its own multisampled framebuffer, `internal/render/msaa.go`), whether the day report opens, and whether the game pauses each night, the on/off ones as check boxes; stored outside the save. The start menu has a winter-sky backdrop, the title, and a DEMO tag in the demo; the in-game menu starts with Resume.

Code: `internal/ui/`, `internal/settings/`, `internal/scene/day_report.go`.

## Log

- 2026-10-01: Top bar, toolbar, event panel, charts, day report, popups, and settings are in.
- 2026-10-11: Games open paused and pause each midnight behind the day report ("Start the next day"; "Pause every night" turns it off) ([[Boreal Playtest]]).
- 2026-10-13: Demo UI pass from QA: Parkitect-style translucent rounded panels, a picture dock and picture submenus from model thumbnails, round centred close buttons, tooltips, a three-pod top bar; Settings works from the start menu (adds full screen); Guest thoughts and Mountain capacity tabs gone (Reviews takes the heart); Ground and Trails views gone from the game (the Trail tool shows trails); the font gains — ° → and friends.
- 2026-10-13: Settings: check boxes for the on/off settings; anti-aliasing off, 2x or 4x (default 4x), live, from a multisampled scene framebuffer instead of the window's.

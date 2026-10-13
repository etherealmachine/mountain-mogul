---
title: Event Feed
kind: system
status: shipped
---

# Event Feed

What the player would otherwise only notice by watching. It doesn't echo the player's own actions (building, opening a lift or the resort); those were dropped 2026-10-13 as unhelpful. The feed is a 256-entry log of:

- avalanches ([[Avalanche]]);
- patrol rescues, one line an hour with the count ([[Ski Patrol]]);
- lift holds for want of snow, and a lift's line passing a 15-minute wait, once a lift a day, suggesting a line attendant if it has none ([[Lifts]]);
- what guests are saying: the day's top complaint when it changes ([[Readable Complaints]]);
- fresh snow overnight, about 8 cm or more ([[Weather]]);
- the day's results (visitors, stars, money in and out, the top complaint), skipped on a day the resort stayed shut, and a record day of visitors;
- credit events ([[Finance]]), guests turned away for want of [[Tickets]] or parking, and goals met or missed.

Clicking an entry with a position moves the camera there.

Alongside it, daily history keeps about two seasons of guests, arrivals, departures, cash, and revenue and costs by category. It drives the charts and the end-of-day report.

Code: `internal/world/events.go`, `history.go`, `internal/sim/events.go`.

## Log

- 2026-10-01: Event feed, daily history, charts, and the day report are in.
- 2026-10-06: Scenario goals met and missed, and the scenario won or lost, appear in light blue ([[Scenario Goals and Rules]]).
- 2026-10-13: The demo QA pass: player-action echoes (builds, lift and resort open/close) dropped; long lines, fresh snow and record days added; rescues folded into an hourly line; closed days get no recap.

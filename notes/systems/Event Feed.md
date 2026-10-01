---
title: Event Feed
kind: system
status: shipped
---

# Event Feed

What the player would otherwise only notice by watching. The feed is a 256-entry log of avalanches ([[Avalanche]]), rescues ([[Ski Patrol]]), lift openings, closings and holds ([[Lifts]]), builds, daily recaps, credit events ([[Finance]]), guests turned away for want of [[Tickets]], and the resort opening or closing. Clicking an entry with a position moves the camera there.

Alongside it, daily history keeps about two seasons of guests, arrivals, departures, cash, and revenue and costs by category. It drives the charts and the end-of-day report.

Code: `internal/world/events.go`, `history.go`, `internal/sim/events.go`.

## Log

- 2026-10-01: Event feed, daily history, charts, and the day report are in.

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
- setup hints, a light tutorial: while the resort has no lifts, no runs off a lift's top, no ticket window, or no parking, the notifications say so in a few words, when the game starts and once a game day until it's built, in build order (`internal/sim/hints.go`);
- credit events ([[Finance]]), guests turned away for want of [[Tickets]] or parking, and goals met or missed.

Clicking an entry with a position moves the camera there.

Alongside it, daily history keeps about two seasons of guests, arrivals, departures, cash, and revenue and costs by category. It drives the charts and the end-of-day report.

Code: `internal/world/events.go`, `history.go`, `internal/sim/events.go`.

## Log

- 2026-10-01: Event feed, daily history, charts, and the day report are in.
- 2026-10-06: Scenario goals met and missed, and the scenario won or lost, appear in light blue ([[Scenario Goals and Rules]]).
- 2026-10-13: The demo QA pass: player-action echoes (builds, lift and resort open/close) dropped; long lines, fresh snow and record days added; rescues folded into an hourly line; closed days get no recap.
- 2026-10-13: Setup hints (no lifts, no runs, no ticket window, no parking); messages wrap over up to three lines; a new game opens with the feed showing.
- 2026-10-13: Renamed Notifications in the game; a red dot on its button while there are unseen ones, and it no longer opens by itself. Setup hints cut to a few words ("No lifts", "No runs off Lift1", "No ticket window", "No parking").
- 2026-10-13: A setup hint comes down as soon as what it asks for is built, checked twice a second even while paused (it had stayed in the feed).
- 2026-10-13: Grooming hints: "No snowcats to groom runs" when groomed runs have no active cat, else "<Run> can't be groomed" for a groomed run no cat has passes on (under trees, or too tight), placed on the run.

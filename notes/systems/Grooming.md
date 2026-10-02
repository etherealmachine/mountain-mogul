---
title: Grooming
kind: system
status: shipped
---

# Grooming

Snowcats live at a maintenance shed and work a painted route split into one section per cat. They park while the lifts run. After closing, each active cat makes one pass of its section that night if any snow-covered cell in it has worn below 90% groomed. A groomed cell becomes packed powder with fresh corduroy, and moguls are knocked down.

[[Skiing]] wears grooming off the lines guests use and builds moguls on ungroomed snow. Guests who like groomed runs score corduroy higher, so grooming feeds [[Satisfaction]]. It changes the [[Snow]] surface but not its depth. A cat grooms whole 5 m cells, so groomed runs show square corners and straight edges where they should curve ([[Hiding the Grid]]).

Cats cost a purchase price and a daily cost that differs between active and standby, which shows up in [[Finance]].

Spec: [[Snow Spec]]. Code: `internal/sim/snowcats.go`, `internal/world/snowcat.go`.

## Log

- 2026-10-01: Sheds, routes, sections, nightly passes, and active and standby costs are in.

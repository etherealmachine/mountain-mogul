---
title: Food Court
kind: amenity
status: partial
---

# Food Court

A lodge service (`ServiceFood`) that sells meals. Part of [[Amenities]].

A meal restores [[Hunger]] to full, holds one seat for 90 seconds, and charges the building's meal price (default $18). Seats are 10 per cell, a number only; there are no tables or chairs yet (see [[Building Interiors]]). A full court turns a hungry guest away. Food-court walls are glazed, and these tiles also count as a place to rest, same as a [[Lounge]].

[[GOAP]] sends a guest here with `RelieveHunger` once hunger is below 0.25. The action is `EatAtFoodCourt`.

Spec: [[Guests Spec]]. Seating, price, and the glazed wall: `internal/world/lodge.go`, `internal/world/lodge_shell.go`.

## Log

- 2026-10-01: Meals, seats, revenue, and the hunger trip are in. Per-counter staffing and door queues are specified in [[Vision]].

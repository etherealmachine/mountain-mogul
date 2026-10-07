---
title: Food Court
kind: amenity
status: partial
---

# Food Court

A lodge service (`ServiceFood`) that sells meals. Part of [[Amenities]].

A meal restores [[Hunger]] and [[Thirst]] to full (it comes with a drink), holds one seat for 90 seconds, and charges the building's meal price (default $18). Seats are 10 per cell, a number only; there are no tables or chairs yet (see [[Building Interiors]]). A full court turns a hungry guest away. Food-court walls are glazed, and these tiles also count as a place to rest, same as a [[Lounge]].

[[GOAP]] sends a guest here with `RelieveHunger` once hunger is below 0.25. The action is `EatAtFoodCourt`.

It also pours drinks: a thirsty guest can stop here for one at the building's drink price (default $8, set in the lodge popup), going in by the food court door, as at a [[Bar]] (`Building.ServesDrinks`). Drinks here count as food revenue. What a bar adds beyond that is still to be decided.

Spec: [[Guests Spec]]. Seating, price, and the glazed wall: `internal/world/lodge.go`, `internal/world/lodge_shell.go`.

## Log

- 2026-10-01: Meals, seats, revenue, and the hunger trip are in. Per-counter staffing and door queues are specified in [[Vision]].
- 2026-10-07: Meals fill thirst too, and the food court pours drinks for thirsty guests.

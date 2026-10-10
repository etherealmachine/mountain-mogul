---
title: Amenities
kind: concept
status: partial
---

# Amenities

Services painted into a [[Lodge Shell]]. Each cell is one service; a connected run of the same service gets a door, and guests walk to the door for the service they want. What a cell does shows on the facade.

In the game now:

- [[Food Court]] — meals, seating, restores hunger
- [[Bar]] — drinks, restores thirst
- [[Lounge]] — a place to sit, restores energy and patience
- [[Tickets]] — day tickets and season passes, the gate on riding lifts
- [[Rental Shop]] — rental skis, the gate on riding for guests who came without

Each service meets guest needs, some rolled per visit (rentals, après at the [[Bar]], warming up in the [[Lounge]]), and lines guests up at the door when full ([[Service Improvements]]). The line stands single file straight out from the door the guest came to, 1 m apart, and turns back beside itself every six guests like a rope maze, on whichever side has more open ground. Guests shuffle up as it moves. Guests using a service are inside: they aren't drawn, can't be clicked, and aren't in anyone's way, and they come back out on their door's step. Every day service the resort could offer, built or not, is collected in [[Services]], each with its own card: [[Snack Stand]], [[Cafe]], [[Restaurant]], [[Demo Shop]], [[Tune Shop]], [[Retail Shop]], [[Ski School]], [[Childcare]], [[Guest Services]], [[Lockers]], [[Shuttles and Parking]], and [[Activities]]; overnight, [[Hotel]], [[Condos]], [[Houses]], [[Budget Lodging]], [[Spa]], [[Night Skiing]], [[Events]], and [[Grocery]]; for staff, [[Staff Housing]].

Guests reach these through [[GOAP]], not by wandering the shell. Planned in [[Next Steps]]: views and a quality level that make an amenity more attractive and let it charge more, and staff who show up to run it.

## Log

- 2026-10-01: Lodge tiles, automatic doors, and the four services above are in. Rental and the rest of the base-area story are not.
- 2026-10-07: Next for services: [[Service Improvements]] (jobs, value, quality, capacity, new types).
- 2026-10-07: Rental shop added; services meet needs and line guests up at the door ([[Service Improvements]]).
- 2026-10-07: Day services listed in [[Services]], a card each.
- 2026-10-10: Lines at the door are single file out from the door, turning back every six guests; guests inside aren't drawn. Before, everyone using or waiting for a service stood stacked on the door's step. Code: `internal/world/door_line.go`, `serveLines` and `tickWaitingForService` in `internal/sim/simulation.go`.

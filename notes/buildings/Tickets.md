---
title: Tickets
kind: amenity
status: partial
---

# Tickets

A lodge service (`ServiceTickets`) that sells the right to ride. Part of [[Amenities]].

[[GOAP]] will not put a guest on a lift until they hold a day ticket or a season pass. With nowhere to buy one, the guest complains that there is no ticket window. Ticket tiles cost more per day to staff than a lounge, and the shell gives them their own facade.

Without a ticket window, [[Demand]] turns away every guest who has no pass. Ticket and pass sales are the main revenue in [[Finance]]. The same popup sets the day ticket price and opens or closes the resort (see [[Calendar]]).

**Cheaper tickets** (`world/tickets.go`): besides the adult day ticket the popup offers three, each a checkbox and a price: kids 0–4 (free by default), children 5–12 (three quarters of the adult price, to the nearest $5), and an afternoon ticket for guests arriving from 12:30 (half price; off by default, the kids' two on). A guest pays the cheapest they qualify for, fixed when they arrive; a kid's age comes from [[Families]]. Free tickets are still collected at the window. A cheaper afternoon brings half-day skiers: arrival chances from 12:30 rise by 1.5 × the share of the adult price saved (75% at half price).

**Night ticket**: from 16:00 at half the adult price by default, while there's a night session ([[Night Sessions]]): a lit lift and closing at least an hour past dusk. Evening guests come for it.

A season pass office, planned with [[Guest Services]], would serve pass holders faster and better than the day-ticket window.

This service does not change [[Hunger]], [[Thirst]], [[Energy]], or [[Patience]]. It gates the ski loop those stats drain on.

Spec: [[Guests Spec]]. Pass pricing as a player decision is in [[Vision]].

## Log

- 2026-10-01: Day tickets, season passes, and the no-window thought are in.
- 2026-10-06: The "no one finds a ticket window" bug wasn't the window: stopped lifts and lifts with no trail for a guest's level were blamed on it ([[GOAP]], [[Demand]]).
- 2026-10-07: A pass office for pass holders planned with [[Guest Services]] ([[Services]]).
- 2026-10-14: Kids 0–4, child 5–12 and afternoon tickets (the user). On the Boreal Goals Test: 930 adults paying $90, 136 children $70, 55 little kids free; with the afternoon ticket at $45, arrivals after 12:30 go from 47 to 85 and ticket revenue from $91,060 to $90,335.
- 2026-10-14: Night ticket and evening guests ([[Night Sessions]]).

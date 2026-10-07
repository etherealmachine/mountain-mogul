---
title: Bar
kind: amenity
status: partial
---

# Bar

A lodge service (`ServiceBar`) that sells drinks. Part of [[Amenities]].

A drink restores [[Thirst]] to full and charges the building's drink price (default $8). The stop is short, ten clock minutes, at the counter (4 a tile). The shell gives bar tiles their own facade.

A bar offers a drink, which fulfils the thirst need: [[GOAP]] sends a guest here once thirst is below 0.25, ahead of more skiing; nearly empty, it outweighs going home, so a guest tries for a drink before leaving. The step is `UseService` with a drink ([[Service Improvements]]). A [[Food Court]] pours drinks too; what only a bar has is après-ski. Some guests arrive wanting it (30%): it tempts them from 14:30, more after a good day, and a guest who's done for the day after that, or still on the mountain when the lifts close, stops at the bar before heading home. Après takes a bar seat (8 a tile a floor) for three-quarters of an hour, costs two drinks, fills thirst, and scores more the better their day went ([[Service Improvements]] step 3).

A bar does not restore [[Energy]] or [[Patience]]. Rest requires a [[Lounge]] or a [[Food Court]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drinks, price, and the thirst trip are in. The afternoon-crowd and staffing beats in [[Vision]] are not.
- 2026-10-07: A food court also serves drinks; the thirst goal now outweighs skiing below 0.25.
- 2026-10-07: A drink takes ten clock minutes (the clock's new scale).
- 2026-10-07: Drinks are an offer fulfilling the thirst need ([[Service Improvements]] step 1).
- 2026-10-07: Drinks are served at the counter (4 a tile); a full bar lines guests up at the door. Each visit scores relief, quality, value, crowding, and the wait ([[Service Improvements]] step 2).
- 2026-10-07: Après-ski: the bar's own job, for guests who arrive wanting it ([[Service Improvements]] step 3).

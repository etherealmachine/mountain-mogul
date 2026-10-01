---
title: Bar
kind: amenity
status: partial
---

# Bar

A lodge service (`ServiceBar`) that sells drinks. Part of [[Amenities]].

A drink restores [[Thirst]] to full and charges the building's drink price (default $8). The stop is short, about 30 seconds, and there is no seat cap. The shell gives bar tiles their own facade.

[[GOAP]] sends a guest here with `RelieveThirst` once thirst is below 0.25. Below 0.05 that goal outweighs going home, so a guest tries the bar before leaving. The action is `RelieveThirstAtBar`.

A bar does not restore [[Energy]] or [[Patience]]. Rest requires a [[Lounge]] or a [[Food Court]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drinks, price, and the thirst trip are in. The afternoon-crowd and staffing beats in [[Vision]] are not.

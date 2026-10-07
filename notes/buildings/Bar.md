---
title: Bar
kind: amenity
status: partial
---

# Bar

A lodge service (`ServiceBar`) that sells drinks. Part of [[Amenities]].

A drink restores [[Thirst]] to full and charges the building's drink price (default $8). The stop is short, ten clock minutes, and there is no seat cap. The shell gives bar tiles their own facade.

[[GOAP]] sends a guest here with `RelieveThirst` once thirst is below 0.25, ahead of more skiing; nearly empty, it outweighs going home, so a guest tries for a drink before leaving. The action is `RelieveThirstAtBar`. A [[Food Court]] pours drinks too; what a bar adds beyond that is still to be decided.

A bar does not restore [[Energy]] or [[Patience]]. Rest requires a [[Lounge]] or a [[Food Court]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drinks, price, and the thirst trip are in. The afternoon-crowd and staffing beats in [[Vision]] are not.
- 2026-10-07: A food court also serves drinks; the thirst goal now outweighs skiing below 0.25.
- 2026-10-07: A drink takes ten clock minutes (the clock's new scale).

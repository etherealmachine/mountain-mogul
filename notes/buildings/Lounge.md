---
title: Lounge
kind: amenity
status: partial
---

# Lounge

The default lodge service (`ServiceLounge`): a place to sit. Part of [[Amenities]]. A cell with no other service painted on it is a lounge.

A lounge offers a seat, which fulfils the rest need: [[GOAP]] sends a guest here when [[Patience]] or [[Energy]] drops below 0.15, and the seat (`UseService`) fills both back up over twenty clock minutes ([[Service Improvements]]). A [[Food Court]] also counts as a rest stop; a [[Bar]] does not. If no rest stop is reachable, the guest emits a thought that the resort needs a lodge.

Only a lounge warms guests up. Some guests feel the cold (40%): outdoors below freezing they chill, faster the colder it is, and once chilled they look for a lounge's fire (a quarter hour, free); chilled through, they think "I'm freezing", and with no lounge, "nowhere to warm up". Any visit indoors warms them too ([[Service Improvements]] step 3).

Spec: [[Guests Spec]]. `OffersRest` is in `internal/world/lodge.go`.

## Log

- 2026-10-01: Rest, the lodge-missing thought, and plain lounge walls are in.
- 2026-10-07: Rest is a seat, an offer fulfilling the rest need ([[Service Improvements]] step 1).
- 2026-10-07: The lounge has 10 seats a tile a floor; a full lounge lines guests up at the door ([[Service Improvements]] step 2).
- 2026-10-07: Warming up: the lounge's own job, for guests who feel the cold ([[Service Improvements]] step 3).

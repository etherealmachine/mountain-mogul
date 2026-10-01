---
title: Lounge
kind: amenity
status: partial
---

# Lounge

The default lodge service (`ServiceLounge`): a place to sit. Part of [[Amenities]]. A cell with no other service painted on it is a lounge.

[[GOAP]] sends a guest here with `Rest` when [[Patience]] or [[Energy]] drops below 0.15. `RestAtLodge` fills both back up over about a minute. A [[Food Court]] also counts as a rest stop; a [[Bar]] does not. If no rest stop is reachable, the guest emits a thought that the resort needs a lodge.

Spec: [[Guests Spec]]. `OffersRest` is in `internal/world/lodge.go`.

## Log

- 2026-10-01: Rest, the lodge-missing thought, and plain lounge walls are in.

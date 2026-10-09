---
title: Large Vehicles
kind: plan
status: idea
---

# Large Vehicles

Buses and RVs: vehicles too long for one 2.5 × 5 m stall. Every car today, vans included, fits a stall ([[Transit]]); these don't, so they need their own parking and their own rules on the road. Listed in [[Next Steps]].

## The idea

- **Buses** bring a big group at once (a ski club, a school trip, a shuttle from town or a hotel) and leave them at a drop-off by the base, not in a stall. A coach is about 12–14 m long and 2.6 m wide and seats about 50, so it could carry several [[Groups]] or one lesson class with room to spare. A bus either waits in a bus bay (a row of long stalls, or a stretch of kerb) until its riders are back, or drives off and returns at a set time, which gives the day a fixed departure that guests plan around.
- **RVs** are a carload that came for several days: 7–12 m long, parked overnight, often in an RV lot with hook-ups that the resort can charge for. They'd be the first guests who don't go home at night, so they touch the [[Calendar]] and lodging (Employee housing and hotels in [[Next Steps]]).

## Parking

- A long vehicle takes two stalls end to end, across a double-loaded row's middle line, or a dedicated long-stall row along one edge of a lot. The lot layout (`world.layoutLot`) would mark which stalls can pair, or the Parking tool would get a bus/RV row option.
- Drop-off zones: a kerb on a road or driveway near the base, where a bus stops, unloads and moves on without taking a stall.
- A lot with no room for a long vehicle turns it away the way a full lot turns cars away.

## On the road

- Following distance and junction clearance scale with length (`carSpacing` is one car length plus 2.5 m today).
- Slower on grades and through lot aisles; turning into an aisle may need the cross aisle's width.
- Rendered from their own models (`models-src/`), like the car kinds (`world.CarKind`).

## Open questions

- Who comes by bus: a share of a scenario's guests set in the editor's Guests tab (beside the group mix), or a bus service the player buys?
- Do buses charge the resort (a coach fee), or bring guests who pay no parking?
- Are RVs worth it before overnight stays exist?

## Log

- 2026-10-09: Noted with the user while adding the car kinds (sedan, mini SUV, SUV, jeep, van, with roof racks and boxes); buses and RVs left for later because they don't fit one stall.

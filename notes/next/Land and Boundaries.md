---
title: Land and Boundaries
kind: plan
status: planned
---

# Land and Boundaries

Where the player may build, where guests may ski, and what's off-limits for good. Extends [[Parcels]]. Listed in [[Next Steps]].

## What's there today

- Parcels are owned, purchasable, or off-limits (`internal/world/parcel.go`). The player builds only on owned land, and buying a parcel pays its price from [[Finance]].
- A rope fence marks the edge of owned land, following cell edges ([[Hiding the Grid]]).
- Ownership is the only boundary. Guests ski wherever the terrain lets them, and nothing separates "inside the ski area" from "beyond it".
- Lift placement only checks that the two stations sit on owned land; the cable can cross anything ([[Lift Operations]]).
- Scenario parcels are rectangles. [[Kirkwood]]'s are a 6 × 6 grid as a stopgap.

## Steps

1. **Ski area boundary.** A boundary line separate from ownership: the area the resort patrols and is responsible for. It starts as the edge of owned land and grows with purchases. Guests outside it are off-piste: no patrol coverage (patrollers from the patrol service, [[Building Services]], don't go there), and injuries there hit the rating hard ([[Ski Patrol]], [[Moments]]). Draw it as boundary rope and signs, smoothed like the parcel fence. Later, gates open backcountry beyond it for experts (already in [[Next Steps]]).
2. **Land purchase.** Make buying land a real decision:
   - hand-drawn parcels in the editor that follow ridges, creeks, and roads instead of rectangles
   - hovering a parcel shows its name, price, area, vertical, and what's on it (trees, steep terrain, road access), with the cost preview from [[Money Feedback]]
   - a purchase event in the [[Event Feed]], and the boundary and fence updating in place
   - prices that can change: land near a growing resort costs more, and a scenario can set prices or make a parcel available only after a date or a goal ([[Scenario Goals and Rules]])
3. **Protected land.** A parcel state (or a flag on off-limits land) for wilderness, national forest without a permit, watershed, or private property. Nothing can be built there and no trees cut; building, lot, and lift placement all check it alongside ownership. Scenarios decide whether lift cables may pass over it and whether guests may ski through. Shown with its own overlay color and a tooltip saying why it's protected.
4. **Protected buildings.** Buildings placed by the scenario (any kind and services from [[Rotated Buildings]] and [[Building Services]]) that the player can't demolish or move, such as a historic inn, a highway maintenance yard, or private cabins. Some can be leased or upgraded instead: a historic lodge kept as a bar, for example. Kirkwood's real inn dates from the 1860s and is a good first one ([[Kirkwood]]).

## Open questions

- Does the boundary come only from owned land, or can a scenario draw a permit area that's larger or smaller than what the player owns?
- Must the whole lift line be on owned land, or can a cable cross unowned land with an easement fee?
- Should guests ever wander outside the boundary on their own, or only through gates?

## Log

- 2026-10-05: Planned from the user's notes: ski area boundary and land purchase, protected land, and protected buildings.
- 2026-10-06: Linked the boundary to the patrol service and protected buildings to the building kinds and services.

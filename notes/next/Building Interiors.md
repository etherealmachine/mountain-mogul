---
title: Building Interiors
kind: plan
status: planned
---

# Building Interiors

Make the inside of a [[Lodge Shell]] readable, then make it look like a lodge. Listed in [[Next Steps]].

## Today

Opening a building's popup draws it as a cutaway: no roof, walls squashed to about 1.25 m. The cell overlay paints each floor cell one flat color per service, and each door shows as a green cell on the ground just outside the wall. That's all there is:

- Nothing on screen says what the colors mean.
- The floor colors don't match the wall tints the same services get on the facade, so a food court is orange on the floor and a different shade on its walls.
- A door marker sits outside the building, which reads as a patch of ground, not a door.
- The interior is empty. A [[Food Court]]'s seats are only a number in the popup, and guests finish their meal, drink, or rest standing at the door.

Code: floor colors and door markers in `appendServiceOverlay` (`internal/scene/service_tools.go`); cutaway in `addLodgeShell` (`internal/render/renderer.go`); wall tints in `Service.Accent` (`internal/world/lodge.go`).

## Steps

1. **Legend.** While a cutaway is open, show a key next to the popup: one swatch per service in the building ([[Lounge]], [[Food Court]], [[Bar]], [[Tickets]]) plus one for doors. The popup already lists tile counts per service; the swatches can sit on those rows.
2. **One color per service.** Floor overlay, wall tint, legend swatch, and the placement ghost all take their color from a single place on `Service`, so the same service looks the same everywhere.
3. **Doors drawn as doors.** Mark each door on its wall, for example a bright threshold strip or an arrow pointing out, instead of tinting the ground cell beyond it.
4. **Furniture, placed procedurally.** Each service fills its tiles with props, chosen by rules and seeded by the building's style seed, in the same spirit as the shell resolving its own walls:
   - Food court: a serving counter along a wall away from the door, tables and chairs filling the rest. Furniture count matches the seats (10 per tile).
   - Bar: a counter with stools, a few high tables.
   - Lounge: sofas and armchairs, a fireplace on an outside wall.
   - Tickets: windows along the wall facing the door, queue rails in front.
   - Every service keeps a clear aisle from its door into the room.
   - Later, the [[Rental Shop]]: a counter, ski racks, a boot bench.

   Props are derived from the tiles and the seed every time the building changes, not saved. Like the shell, re-rolling the style seed re-rolls them. They come from the [[Model Pipeline]] and draw as static instances only for the building in cutaway, so they cost nothing on the rest of the map.
5. **Guests inside.** Seated diners take real chairs. A guest eating or resting is drawn at their seat in the cutaway instead of standing at the door, and the popup's diner count matches the filled chairs.

Steps 1–3 are small and could ship together. Step 4 is the big one; step 5 depends on it.

## Open questions

- Should furniture define capacity (seats = chairs that fit) instead of the flat 10 per tile? That would make layout matter for gameplay.
- Do props show only in the cutaway, or also through glazed food-court walls from outside?
- Do guests path inside the building, or jump from door to seat?
- How does placement change once the shell has storeys?

## Log

- 2026-10-01: Documented the cutaway and overlay as they are, and planned a legend first, procedural furniture later. Nothing implemented yet.

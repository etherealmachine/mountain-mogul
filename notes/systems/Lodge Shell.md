---
title: Lodge Shell
kind: system
status: partial
---

# Lodge Shell

How a service building gets built and how it looks. Each building has its own 5 m grid, placed anywhere and turned to line up with the nearest road or lot (R turns it). The player drags out a rectangle of that grid as empty floor, then puts services in its tiles ([[Building Tool]]); the exterior resolves itself from the footprint. Walls, corners, windows, doors, and a hip roof come from each tile's neighbours on a 2.5 m half-cell grid, and a style seed varies the window rhythm and tint. The ground under the shell is graded flat to the first tile's floor height.

Every connected run of one service gets a door on an outside wall that opens onto walkable ground: toward the parking lot for [[Tickets]], toward the nearest lift otherwise. Guests walk to the door of the service they want (see [[Pathfinding]]). Each service shows on the facade: a [[Food Court]] is glazed, a [[Bar]] and [[Tickets]] have their own fronts, a [[Lounge]] has plain or windowed walls. While its popup is open, the building draws as a cutaway with the floor colored by service and a green mark outside each door. There is no legend for those colors and no furniture inside; [[Building Interiors]] plans both.

Tiles come from the [[Model Pipeline]] kit and draw as static instances in [[Rendering]].

Not built yet, from [[Vision]]: storeys, a style palette choice, and a shuffle button. VISION has the player marking doors; the code replaced player-placed doors with automatic ones, and older saves' door cells are ignored on load.

Code: `internal/world/lodge.go`, `lodge_shell.go`, `internal/scene/service_tools.go`.

## Log

- 2026-10-01: Painted tiles, automatic doors, facade by service, roofs, grading, and cutaway are in. Storeys and a style picker are not.
- 2026-10-06: Planned: a rotated grid and shed / tent / lodge kinds ([[Rotated Buildings]]), and services as one model ([[Building Services]]).
- 2026-10-06: Service buildings have their own rotated grid ([[Rotated Buildings]] step 2); buildings saved in older forms no longer load.
- 2026-10-06: Three kinds of building, each with its own kit, costs, and palette: shed, tent, and lodge (up to three storeys) ([[Rotated Buildings]] step 3).
- 2026-10-06: Services come from one registry, adding ski patrol and the snowcat garage ([[Building Services]]).
- 2026-10-08: Buildings go up empty (dragged out as a rectangle) and services go in afterwards from their own menu; each room has its own popup ([[Building Tool]]).

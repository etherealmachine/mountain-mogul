---
title: Lodge Shell
kind: system
status: partial
---

# Lodge Shell

How a service building gets built and how it looks. The player paints cells and picks each cell's service; the exterior resolves itself from the footprint. Walls, corners, windows, doors, and a hip roof come from each tile's neighbours on a 2.5 m half-cell grid, and a style seed varies the window rhythm and tint. The ground under the shell is graded flat to the first tile's floor height.

Every connected run of one service gets a door on an outside wall that opens onto walkable ground: toward the parking lot for [[Tickets]], toward the nearest lift otherwise. Guests walk to the door of the service they want (see [[Pathfinding]]). Each service shows on the facade: a [[Food Court]] is glazed, a [[Bar]] and [[Tickets]] have their own fronts, a [[Lounge]] has plain or windowed walls. While its popup is open, the building draws as a cutaway with the floor colored by service and a green mark outside each door. There is no legend for those colors and no furniture inside; [[Building Interiors]] plans both.

Tiles come from the [[Model Pipeline]] kit and draw as static instances in [[Rendering]].

Not built yet, from [[Vision]]: storeys, a style palette choice, and a shuffle button. VISION has the player marking doors; the code replaced player-placed doors with automatic ones, and older saves' door cells are ignored on load.

Code: `internal/world/lodge.go`, `lodge_shell.go`, `internal/scene/service_tools.go`.

## Log

- 2026-10-01: Painted tiles, automatic doors, facade by service, roofs, grading, and cutaway are in. Storeys and a style picker are not.

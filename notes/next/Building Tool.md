---
title: Building Tool
kind: plan
status: shipped
---

# Building Tool

Make building a resort's buildings read as two steps: build a shell, then fit out its rooms. Today the Buildings menu (Amenities in the game) mixes the two in one list: Lodge, Tent, and Shed say what a new building is built as, and the services say what a tile is, but a building only starts when a service is painted on open ground, so picking "Shed" alone does nothing visible. Builds on [[Rotated Buildings]] and [[Building Services]]; the shell is [[Lodge Shell]].

## Today

- One submenu holds Lodge, Tent, Shed (the kind for the next new building) and Lounge, Food court, Bar, Tickets, Ski patrol, Snowcat garage (the service to paint).
- Clicking open ground with a service starts a building of the chosen kind with one tile of that service; clicking a wall adds a tile, a roof switches its service, a right click removes it.
- Every tile has a service (a new tile is lounge if nothing else); there's no empty floor.
- A tile's price is the service's price scaled by the kind, so a building's cost is hard to read as "the building" plus "what's in it".
- Clicking a building opens its popup (storeys, vehicles, prices) but you can't change its rooms from there.

## Proposal

1. **Shell first.** The Buildings menu lists only Lodge, Tent, and Shed, each with what it is (heated or not, storeys, price per tile). Picking one drags out a rectangular footprint on its own grid, turned to the nearest road or lot (R turns it), shown as a ghost with its walls and roof and the price; click to build, Esc to cancel, as with parking lots. More rooms come by dragging out from a wall later, again as a ghost; built buildings don't turn or move.
2. **Empty floor.** A new shell's tiles start empty: walls, roof, and heat, but no service and no staff. Costs split in two: the structure (per tile, by kind) when the shell goes up, and the fit-out (per tile, by service) when a room is assigned, each shown separately.
3. **Rooms in the building's panel.** Clicking a building opens its panel with the cutaway view (already there) and a palette of services; painting tiles inside the building assigns or clears rooms. The panel lists each room with its tiles, what it does (seats, patrollers, garage space) and its daily cost, and keeps today's controls (storeys, prices, vehicles, style, delete). Services leave the toolbar entirely.
4. **Hints.** The palette says what a service needs: tickets near the parking, patrol and the garage on snow for snowmobiles and cats, food and lounge for tired skiers, and greys out what the kind doesn't suit, if we decide kinds limit services (open question in [[Rotated Buildings]]).
5. **Same in the editor**, free and on any land.
6. **Later: presets.** One-click starters like "ticket booth" (a one-tile tent with tickets) or "patrol shed" (a shed with patrol and a 2 × 2 garage), placed as a ghost.

## Decisions

From the user, 2026-10-08: buildings and services in **two separate menus** (not services in the building's panel, as proposed above); a building must be placed first, then services go in it; each service is selectable on its own with its own popup, and the building has its own popup with its actions and the list of its services. Footprint: **drag a rectangle**. Selecting: **building, then room**.

## Built (2026-10-08)

- **Empty floor.** A tile can have no service (`ServiceNone` in `Building.Tiles`, kept on save). Empty floor gets no door and costs no staffing. A **room** is a connected run of one service's tiles (`Building.Rooms`, `RoomAt`), each with its own door as before.
- **Costs split.** Structure is $10,000 a tile in a lodge (scaled by kind, per storey; `ShellKind.StructureTileCost`) plus the kind's base cost for a new building (`world.ShellCost`); fit-out is the service's old tile price less the structure (`FitOutTileCost`, `FitOutCost`), so a tile with a service costs what it did. A storey costs every tile's structure and fit-out again. Taking a service out refunds nothing.
- **Buildings menu** (Lodge, Tent, Shed): drag out a rectangle of floor on the building's own grid (up to 12 × 12, turned to the nearest road or lot; R turns it), shown as a ghost with its price; click inside it or press Enter to build, Esc drops it. A drag that starts beside a building adds floor to it (in its kind). Right-click a tile to remove it; the last tile removes the building.
- **Services menu** (Lounge, Food court, Bar, Tickets, Ski patrol, Snowcat garage, Rentals): click or drag over a building's tiles to put the service in; right-click empties a tile. Clicking open ground says to build first.
- **Building popup:** what it's built as, storeys, floor (tiles, how many empty), a button per room, daily cost, inbound guests, new style, delete. While it's open the building is a cutaway coloured by room.
- **Room popup** (from the building's list, or a click on the room in the cutaway): its tiles and door, the service's own controls (meal, drink, and rental prices; diners; patrollers; garage space and vehicles; the resort's hours and prices at tickets), its daily cost, Remove (back to empty floor), Back to building. The room is highlighted in the cutaway and the rest dimmed.
- **Editor:** the same two menus and tools, free and anywhere; its building popup lists rooms and empty floor.
- Code: `internal/scene/service_tools.go` (tool session, popups), `internal/world/lodge.go` (empty floor, rooms, costs).

Checked on the Goals Test save with a throwaway program: an empty 12-tile lodge has no rooms or doors and isn't used by guests; a food court and a ticket room each get a door; extending, removing a tile, and save and load keep empty floor. Found and fixed a hang where empty floor's flood fill walked off the building. Not checked by hand: dragging, painting, and the popups in play.

## Open questions

- Should empty floor cost upkeep (heating) or count toward anything (seating when a lodge has no lounge)?
- Does a room need an outside wall for its door, and should the palette warn when it doesn't have one?
- Drag a rectangle for the shell, or paint tiles freely as today? A rectangle is simpler and matches lots; painting allows L shapes (extending from a wall covers most of that).
- Keep a quick path (one click with a service on open ground) for experienced players?

## Log

- 2026-10-06: Written up after the user found the mixed Buildings menu confusing: shell first, then rooms in the building's panel.

- 2026-10-08: Built with the user's decisions: Buildings and Services menus, empty floor, rectangle footprint, building and room popups.

---
title: Building Tool
kind: plan
status: idea
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

## Open questions

- Should empty floor cost upkeep (heating) or count toward anything (seating when a lodge has no lounge)?
- Does a room need an outside wall for its door, and should the palette warn when it doesn't have one?
- Drag a rectangle for the shell, or paint tiles freely as today? A rectangle is simpler and matches lots; painting allows L shapes (extending from a wall covers most of that).
- Keep a quick path (one click with a service on open ground) for experienced players?

## Log

- 2026-10-06: Written up after the user found the mixed Buildings menu confusing: shell first, then rooms in the building's panel.

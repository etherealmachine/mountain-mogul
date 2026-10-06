---
title: Rotated Buildings
kind: plan
status: in progress
---

# Rotated Buildings

Buildings placed anywhere and turned to any angle, still painted tile by tile on a 5 m grid, but a grid in the building's own frame rather than the map's. Three kinds of building to start: shed, tent, and lodge. First of three steps the user set on 2026-10-06: this, then [[Building Services]], then fixing how services work. Narrows [[Gridless Drawing]] for buildings; the shell itself is described in [[Lodge Shell]].

## Today

- A service building is painted cell by cell on the map's 5 m grid. Each cell picks a service (lounge, food, bar, tickets); walls, corners, windows, doors, and a hip roof resolve from each tile's neighbours on a 2.5 m half-cell grid. Doors are automatic. The ground is graded flat to the first tile's floor.
- The equipment shed and the patrol hut are separate point-placed meshes (`BuildingShed`, `BuildingPatrolHut`) on their own code paths. Point-placed lodges, bars, and ticket offices from older saves and testbeds go through `ConvertLegacyBuilding`.
- Parking lots are already rectangles placed freely ([[Transit]] step 3), but loading still converts painted-cell lots to their bounding box.

## Decisions

Made with the user on 2026-10-06:

- **A 5 m grid, rotated.** Each building has an origin and a rotation; its tiles sit on a 5 m grid in that frame (the 2.5 m half-cell grid for walls and roofs stays). Tile painting stays: the player paints cells in the building's frame, not the map's.
- **Three kinds of building, in rising cost:**
  - **Shed**: unheated, one storey.
  - **Tent**: heated, one storey.
  - **Lodge**: heated, up to three storeys.
  They differ in cost and look from the start. What heated and unheated mean for guests, and how much nicer a lodge is than a tent, come later.
- **No legacy code.** Old buildings and painted lots are removed from saves rather than converted; they're re-added by hand. That includes taking out the lot conversion added in [[Transit]] step 3.
- **The equipment shed and snowcats join the same model** (as a service, see [[Building Services]]).

## Steps

1. Done: **Drop the legacy paths.** Loading skips lots without a rectangle, service buildings without tiles, and point-placed bars and ticket offices; the painted-lot conversion, the painted-interior (food-court cells) load, and the old-mesh conversion are gone, along with their tests. Placing a lodge, bar, or ticket office by a point (tests, testbeds, the editor's ticket office) still makes a small service building there. Boreal and Kirkwood had nothing in an old form, so neither file changed. The equipment shed and patrol hut are gone too (the user's call): no build tools in the game or editor, the game's Operations menu with them, and saved ones (with their cats and patrollers) are skipped on load. They come back as services in [[Building Services]]; until then nothing grooms and nobody patrols. The snowcat and patrol sims stay, ready to hang off those services, and testbeds can still place a shed.
2. Done: **The rotated frame.** A service building has an `Origin` and `Rotation`; its `Cells`, `Tiles`, and `Doors` are in its own 5 m grid, and `Ground` is the map cells under it (a map cell counts when its centre, or a point 1.5 m in from a corner, is on a tile). Walls, roofs, and doors resolve in the grid and are placed in the world turned with it. Ground drives walking, grading, plowing, overlap, the trail graph, and doors (a door opens onto the map cell in front of it). In the game's Amenities tools a new building lines up with the nearest road or parking lot within 120 m (R turns it); clicks are ray-picked in each building's own grid, and a tile is legal when every map cell it would cover is owned, walkable, and free. Lots moved onto the same `Ground` field. Saves carry `ox`, `oz` and the rotation for each service building.
3. Done: **Shed, tent, and lodge.** `world.ShellKind` on each service building (saved as `kind`, with `storeys`). The Amenities menu has Lodge, Tent, and Shed buttons that set what new buildings are built as. Each kind has its own kit: the OpenSCAD kit takes a `kind` (`models-src/{lodge,tent,shed}_*.scad`), giving the tent 3 m fabric walls with clear vinyl panels, a tied-back doorway, aluminium corner posts, and a steep fabric roof; the shed 4 m corrugated metal on a concrete sill, high windows, a vent, a roll-up door, and a low metal roof; the lodge as before (5 m). Palettes are per kind (fabric whites; galvanised, barn red, green, or slate metal). Costs (guesses, untuned): base $50,000 / $15,000 / $10,000; tiles at 100% / 40% / 25% of the service's price; daily base $200 / $120 / $60 and per-tile upkeep at 100% / 80% / 50%. A lodge's popup has a Storeys stepper (1–3): adding one pays for every tile again, taking one off refunds nothing; walls and corners repeat per storey (doors on the ground floor only), the roof sits on top, and the cutaway shows the ground floor. Food-court seats and upkeep scale with storeys. Shown on Boreal by a shed, a tent, and a three-storey lodge placed from a script.
4. **Rebuild Boreal's buildings** by hand in the editor once [[Building Services]] is in.

## Open questions

- Which services each kind may hold: tickets or food in a tent, a snowcat garage only in a shed?
- How storeys add up: is a second storey the whole footprint, or painted per cell; does it add capacity to the services below or hold services of its own?
- Rotation steps: free, or snapped (15°) with the road-aligned default?
- Whether the shell's grading (flat to the first tile's floor) works on steep ground once buildings can turn.

## Log

- 2026-10-06: Planned with the user: a rotated 5 m grid with tile painting, shed / tent / lodge kinds, and no conversion of old buildings or lots.
- 2026-10-06: Steps 1–2: old-form buildings and lots are dropped on load (no conversion); service buildings have their own rotated 5 m grid, lined up with the nearest road or lot when placed. The shed and patrol hut stay until [[Building Services]].
- 2026-10-06: Removed the equipment shed and patrol hut from the build tools and saves, at the user's call; grooming and patrol return as services.
- 2026-10-06: Step 3: shed, tent, and lodge kits from one parametric OpenSCAD kit, per-kind costs and palettes, and lodge storeys (1–3) from the popup.

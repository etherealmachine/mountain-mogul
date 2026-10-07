---
title: Scenario Editor
kind: tooling
status: shipped
---

# Scenario Editor

A separate scene for authoring the scenarios New Game starts from, with no simulation running. It has the same build tools as play (lifts, buildings, roads, terrain brushes), plus:

- [[Parcels]]: drag rectangles to create or edit parcels and set state and price
- automatic [[Snow]] cover from snowline, elevation, slope, curvature, drainage, wind aspect, and treeline
- automatic forest from layered noise and drainage, thinned above treeline and on cliffs, placed as individual spaced trees (see [[Trees]])
- trails: the Trail button starts a new green run and paints it (drag; right-drag erases; Esc finishes, and a run left empty is removed); clicking a run with no tool opens its popup (name, difficulty, groomed, Add and Remove cells, Clear). Every run shows in the overlay, the one being edited brighter. Painting, colours, and the popup are shared with the game (`scene/trail_tool.go`, `editor_trails.go`).
- the ski-area boundary: the Ski Area button draws outlines corner by corner (click to place, click the first corner or Enter to close, a right click without panning takes back a corner or removes an outline, Esc drops the outline in progress), over the OpenStreetMap layer as a guide; O copies OpenStreetMap's own ski-area boundary where the import has one (Boreal's doesn't). The area shows as a lavender wash with a stronger edge, and the ground inside is the resort's skiable terrain (`world.SkiArea`, saved as `ski_area`; `scene/editor_ski_area.go`).
- ground brushes in the Terrain menu: Smooth, Flatten, Raise, and Lower, with Radius and Strength sliders, applied while the mouse is held (`world.Terrain.ApplyBrush`, `scene/terrain_brush.go`). They edit the true ground on the 1.25 m detail lattice where the map has one (else the 5 m cells): the cells under the brush are rebuilt from it as the import builds them, and the detail around them re-derived, so ground outside the brush doesn't move. Smooth averages over a third of the radius, so a wide brush takes out wide features like Boreal's half-pipe; Flatten pulls toward the height where the stroke began; Raise and Lower move up to 2 m a second at full strength. Rebuilding the terrain layers starts from the import again and loses these edits.
- a Layers panel (Layers button) for an imported map: the base import, which reopens the import to change the square (Change...) or imports the same square again in the background with the same grid and layer settings (the ↻ reload button, which asks first when there's anything to lose), then each terrain layer with a checkbox and a strength slider: the ground layers, then Auto trees and Auto snow, which keep what's built ([[Terrain Layers]]). Last is an OpenStreetMap checkbox, with a Refresh button that fetches the import's OpenStreetMap features again for the same square without touching the ground (for imports made before a feature was fetched, or after OpenStreetMap changes; the roads, creeks, and lakes layers use them next time they run), visual only and off on opening, that drapes the import's real-world features on the ground with labels, for lining up what's built: lifts in red with a pad at each station and their kind ("quad chair"), runs coloured by difficulty, roads in amber at their paved width, and the ski-area boundary in purple. It's drawn under everything built (roads, parking, buildings, lifts, and placement ghosts all sit on top), though labels draw over everything. Labels skip any that would overlap, lifts first. The ribbons are redraped twice a second, so they follow layer rebuilds and snow. Auto snow follows the start date.
- the scenario's start date, which sets day one of the [[Calendar]]
- a Scenario details dialog (Details button or the escape menu) for the name, location, difficulty, campaign order, tutorial flag, and a multi-line description ([[Scenario Metadata]])

Files open and save in `assets/scenarios/` in the [[Save Format]], with an unsaved-changes marker in the title. New maps come from [[Terrain Import]]. What it makes are [[Scenarios]]; the title shows the display name and the file name.

Code: `internal/scene/editor*.go` (`editor_layers.go` for the Layers panel, `editor_osm.go` for the OpenStreetMap overlay), `world_layers.go`, `snowgen.go`, `forestgen.go`, `autogen.go`.

## Log

- 2026-10-01: Editor scene, parcels, snow and forest generators, start date, and file handling are in.
- 2026-10-02: The forest generator and the plant and glade brushes work on stored trees ([[Stored Trees]]).
- 2026-10-02: Scenario details dialog for names, descriptions, and campaign order.
- 2026-10-06: Layers panel for imported terrain; a new import keeps the scenario's name, details, and start date.
- 2026-10-06: A strength slider on every layer row; timings dropped.
- 2026-10-06: Auto trees and Auto snow layers. A new import now starts on the season's opening day, in the season the editor was set to. The Auto tool's treeline slider starts from the climate.
- 2026-10-06: OpenStreetMap overlay in the Layers panel: lifts, runs, roads, and the boundary draped on the ground with labels.
- 2026-10-06: The OpenStreetMap overlay draws water in cyan (streams and lake outlines, with names); the Layers panel is wider to fit the counts.
- 2026-10-06: A ↻ reload button beside Change... re-imports the base's square in place, for fresher data or what newer imports fetch (water, for older maps).
- 2026-10-06: The Scenario details dialog has a Goals tab for scenario goals and rules ([[Scenario Goals and Rules]]).
- 2026-10-06: The Buildings menu is the game's building tool, free and on any land: Lodge, Tent, or Shed, then a service to paint (lounge, food court, bar, tickets, ski patrol, snowcat garage), R to turn a new building, right-click to remove a tile. Clicking a building with no tool opens its popup (storeys, garage vehicles, new style, delete). The old point-placed Lodge and Tickets tools are gone.
- 2026-10-07: Ground brushes: Smooth, Flatten, Raise, Lower ([[Terrain Realism]] step 2).
- 2026-10-07: Trails: paint and edit runs, as in the game, so test scenarios can start with runs (for checking the [[Demo]]'s goals).
- 2026-10-07: Ski-area boundary tool ([[Demo]] step 2).
- 2026-10-07: OpenStreetMap Refresh in the Layers panel; the row counts ski areas. Boreal's square has no mapped ski-area boundary, so it's drawn by hand.
- 2026-10-07: The ski-area boundary shows in the game as a rope line: orange poles every 8 m along the outline with a drooping yellow rope, built with the parcel fence (`buildSkiAreaFence`, `scene/ski_area_overlay.go`). It leaves gaps at buildings, parking lots, and roads, is rebuilt every clock hour so it rides the snow (the parcel fence too), and is lit by the frame's light (the debug shader's new `uTint`), so it dims at night. The editor shades it from a cell mask the world caches until the boundary changes.

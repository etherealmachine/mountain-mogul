---
title: Trail Network
kind: plan
status: partial
---

# Trail Network

Trails drawn as lines of nodes instead of painted cells: a run starts on a lift top (or a building, or another run) and ends on a lift base, each node with its own width so a run can open into a wide face or pinch through a gap. Ungroomed terrain (glades, bowls, backcountry) is outlined as an area instead. Replaces painting in [[Trails]]; one case of [[Gridless Drawing]] (the shape is the truth, cells are derived).

## Why

Read on 2026-10-08:

- **Painting is clunky.** A trail is the cells a 2-cell brush has touched (`scene/trail_tool.go`), so edges are stair-stepped and widths uneven, and the overlay draws one colour per 5 m cell ([[Hiding the Grid]]).
- **Connections are accidental.** A trail connects to a lift only if its cells cover the exact cell of the lift's top or base (`world.BuildTrailGraph`), to a building through its door cell, and to another trail by touching it. Nothing tells the player whether a run is connected.
- **Most of the game reads only the cells.** Guests don't ski along trails: steering heads for the destination and only detours round trees (`sim/ski_route.go`). A trail matters through its cells touching lifts, buildings, and other trails (the graph [[GOAP]] plans over); its cell set (grooming sections, terrain capacity, demand's terrain mix, conditions, run judging via `TrailAt`); and its difficulty (which lifts a guest uses). So a drawn trail can be rasterised to cells and almost everything downstream stays as it is.
- **No home for ungroomed terrain.** Glades, bowls, and backcountry are just "off trail": runs mostly off trail get no difficulty verdict, and nothing marks a glade as a place to ski.
- **Found along the way:** placing or removing a lift doesn't rebuild the trail graph (`PlaceLift`, `RemoveLift`), so a lift dropped on a trail doesn't connect until something else rebuilds it. And [[Trails]] said leaving the trails costs beginners more; that cost isn't in the code (the free-roam actions in `goap/action.go` say they're penalised but aren't).

## Decisions

From the user, 2026-10-08:

- **Node-based trails**, connected directly to lifts.
- **Width per node**, tapering between nodes.
- **Polygon zones** for ungroomed areas (glade, bowl, backcountry), outlined corner by corner like the ski-area boundary.
- **Redraw by hand.** Painted trails are dropped (old saves lose them); Boreal and Kirkwood get their trails redrawn in the editor, with the OpenStreetMap runs as a guide.

## The model

**A trail is either a run or a zone**, one type so every consumer keeps working:

- `Trail.Kind`: run, glade, bowl, or backcountry.
- A run has `Nodes`: each a world position and a width (metres, say 10–80). Its shape is a smooth centre line through the nodes (a Catmull-Rom curve, so bends aren't kinks), with the width interpolated along it.
- A zone has an `Outline`: a polygon of world positions.
- Name, difficulty, and (runs only) groomed, as now. Zones are never groomed.
- **Ends attach.** A run's first and last nodes can be attached to a lift top or base, a building, or a node or segment of another trail; an attached end follows what it's attached to (a lift moved, a junction node dragged). Attachment is what the tool shows the player; the graph still derives from cells, which an attached end always covers, so `BuildTrailGraph` needn't change at first.
- **Cells are derived**, never saved: a cell belongs to the trail when its centre falls inside the run's ribbon or the zone's polygon (plus the cells under attached ends, so a narrow run still touches its lift's cell). Recomputed when the shape changes; `Trail.Cells`, `TrailAt`, and everything reading them stay.

**Grooming keeps clear of trees** (the user, 2026-10-08). A run's ribbon can reach into forest, especially where it's widened, but cats only groom open snow. Today they don't check: the pass planner (`newPassPlanner`, `sim/groom_passes.go`) takes every trail cell with snow, trees or not. With this plan:

- The groomable part of a run is its ribbon less a clearance round every trunk: half the tiller's width plus a margin, read from the terrain's trunk field (`world/trunk_field.go`, built for steering), so it's per trunk, not per cell.
- Pass planning keeps the tiller inside that area: passes stop short of trunks and route round tree islands, and corduroy and grooming are only stamped where the tiller went (`GroomMap`, `Cell.Grooming`).
- A cell that's mostly under trees stays ungroomed, so its snow stays natural and guests' grooming taste reads it as off-piste.
- The run popup shows how much of the run is groomable ("82% groomable; the rest is under trees"), and the ghost shades the tree-covered stretch while drawing.
- Zones are never groomed.

**Drawn as shapes.** Runs as a filled ribbon in their difficulty colour with a darker edge line, draped on the terrain like the editor's OpenStreetMap ribbons (`scene/editor_osm.go`); zones as an outline with a light wash and a hatch by kind, like the ski-area boundary. Names along the line. The per-cell trail overlay goes.

## Tools

**Run tool** (Trails menu, game and editor):

- Click to start: snaps to a lift top, a building, or another trail when near one (highlighted), otherwise starts loose.
- Each click adds a node; the ghost shows the ribbon at the current width, its difficulty colour, and its length and drop.
- Finish by clicking a lift base, building, or trail (snapped and attached), or Enter / double-click to end loose. Esc drops it.
- Editing a run (select it): drag a node to move it; drag a node's edge handle to set its width (or scroll over the node); click a segment to insert a node; right-click a node to delete it. Detach or re-attach an end by dragging it.
- The popup keeps name, difficulty, groomed, groom %, and delete, and adds length, vertical drop, average and steepest pitch, and what each end attaches to ("Lift 1 top → Lift 3 base"), with a warning when an end is loose.

**Area tool:** pick glade, bowl, or backcountry; click corners, click the first or press Enter to close; drag corners afterwards. Its popup: name, kind, difficulty, size, delete.

## Steps

1. Done: **Fix the lift rebuild.** `PlaceLift`, `RemoveLift`, and moving a lift rebuild the trail graph, so connections follow lifts now (worth doing before anything else).
2. Done: **Model, rasterisation, and grooming.** `Trail` gets `Kind`, `Nodes` (with widths), `Outline`, and end attachments; cells are derived from the shape. Saved as shapes; painted trails dropped on load. Testbeds' `PolylineCells` trails become runs (a centre line and width is what they already are). The pass planner grooms only the groomable area (the ribbon less the trunk clearance, above). Check: the derived cells of a run snapped between a lift top and base connect it in the graph; capacity and demand read the same as for an equivalent painted trail; on a run drawn through a glade, no pass point is within the clearance of a trunk and only the open snow is groomed.
3. Done: **Run tool and drawing.** The node tool with snapping and attachment, node and width editing, the ribbon and labels; the trail popup's new readouts. Game and editor.
4. **Areas.** The zone tool, drawing, and popup. Zones join the graph as trails do; never groomed; run judging counts time in a zone as on its difficulty (so a glade lap gets a verdict).
5. **Redraw Boreal and Kirkwood** in the editor over the OpenStreetMap runs, and check guests use them (headless day: runs per trail, lift use).

## Built (2026-10-08)

- **Lifts rebuild the graph** when placed, removed, or moved (`PlaceLift`, `RemoveLift`, `structure_edit.go`).
- **Model** (`world/trail_shape.go`): `Trail.Kind` (run, glade, bowl, backcountry), `Nodes` (position and width, 8–80 m, default 25), `Start`/`End` attachments (`TrailEnd`: lift top or base, building, trail), `Outline` for areas. The centre line is a Catmull-Rom curve sampled every 2.5 m with the width interpolated. `ShapeTrail` derives the cells (centres within the half-width, the cells the line crosses, and the cells under attached ends); `RebuildTrailGraph` reshapes every trail first, so ends attached to a lift follow it, and an end whose lift or building is gone comes loose. Saved as shapes (`TrailData.Nodes`, `Start`, `End`, `Outline`); painted trails are dropped on load. Testbeds' trails are runs (`runTrail`, `groomedRun`, `groomedRect`).
- **Grooming keeps clear of trees**: a pass point must be 3 m (half the tiller and half a metre) from the nearest trunk, read from the trunk field (`Terrain.TrunkClear`, `NearestTrunk2`); seeds and corner stabs are held to it too (`groom_passes.go`).
- **Run tool** (`scene/run_tool.go`, game and editor, the Trail button) draws new runs: click near a lift end, door, lot, or trail to start (it snaps within 15 m and attaches), click to add nodes, click a lift base, building, or trail to finish (attached), or Enter to end loose; [ and ] set the next node's width; right-click takes back a node; Shift-click places a node that doesn't join the trail under it (for several runs off one lift top); Esc drops the run. New runs are groomed. **Editing** is by selection: click a run with no tool to open its popup, and while it's open its node and width handles show and clicks on them or on the run go to it first: drag a node (an end re-attaches where it's dropped), drag a width handle, click the run to insert a node, right-click a node to delete it, [ and ] over a node to change its width. Clicks elsewhere select as usual, so overlapping runs are edited one at a time.
- **Drawing**: trails are draped ribbons, a see-through fill in the difficulty colour (lighter when groomed) with edge lines, in two renderer layers (`SetTrailLayer`): the settled trails, redrawn when trails change or every 3 s for the snow, and a live layer for the ghost, handles, and the run being edited. The per-cell trail overlay is gone. Names stay at the centroid.
- **Popup**: shape (length, drop, average and steepest pitch), what each end is attached to, a note when an end is loose, Delete. While it's open the run is selected: its handles show and clicks on it edit it.

Checked on the Goals Test save with a throwaway program: a three-node run attached from Lift 1's top to its base connects in the graph and the lift serves its difficulty; moving the lift's base drags the end; save and reload keep nodes, ends, and grooming; across 9,539 cat pass points none comes within 3 m of a trunk (the first try had 77, from seeds and corner stabs, now fixed); screenshots show the ribbon, and corduroy stopping short of the trees. Not checked by hand: drawing and editing in play.

**Until step 5, Boreal and Kirkwood have no trails** (their painted ones are dropped): lifts serve every difficulty (the no-trail fallback), guests ski free, and cats have nothing to groom.

## Follow-ups (not in this plan)

- **Guests follow runs**: a guest on a run steers along its line instead of straight at the lift, keeping to the ribbon, with leaving it costing beginners more (what [[Trails]] claimed). The centre line makes this possible; it changes how guests ski, so it's its own plan ([[Skiing]]).
- **Graph from attachments** instead of cell overlap, including junctions mid-run.
- **Closures and slow zones** become natural on a node line (close a run, mark a stretch slow).
- **Backcountry gates** at the boundary, no patrol beyond ([[Land and Boundaries]]).
- **Difficulty suggestion** from the run's steepest sustained pitch.
- **Import from OpenStreetMap** as a starting point (not chosen for Boreal and Kirkwood, but cheap once runs are lines).

## Open questions

- Smooth curves through the nodes, or straight segments with rounded joins?
- Width limits, and does width cost anything (cutting and grading a wider run)?
- Should drawing a run through forest offer to cut the trees in its ribbon (a cost, like the glade tool), or leave them as islands the cats go round?
- Can a run fork mid-way (a run that starts on another run's segment), or only meet at ends?
- Does a zone overlapping a run take the run's cells (a glade beside a groomer), or do cells belong to both, as overlapping painted trails do today (hardest wins in `TrailAt`)?

## Log

- 2026-10-08: Planned with the user: node trails with widths per node, polygon zones, redraw Boreal and Kirkwood by hand. Unranked.
- 2026-10-08: The user: cats must not groom under trees. Added the groomable area (ribbon less trunk clearance); the user folded it into the trail update (step 2) rather than fixing today's trails separately.
- 2026-10-08: Steps 1–3 built: lifts rebuild the graph, runs as nodes with widths and attached ends, grooming clear of trunks, the run tool and drawn ribbons. Areas and the redraw of Boreal and Kirkwood are next.
- 2026-10-08: Shift starts a new run and keeps its nodes from joining other trails, for several runs off one lift.
- 2026-10-08: From the user: the Trail tool now only draws new runs; a run is edited by selecting it (its popup open, no tool), when its handles show and clicks on them or on the run go to it before anything else, so overlapping runs can each be edited (the popup's Edit shape is gone; Shift while drawing still places a node that doesn't join the trail under it). Ribbons no longer clip into the terrain: draped on the drawn surface (`render.VisualElevationAt`, moguls included), the fill split into strips 4 m wide, and drawn with a depth offset.
- 2026-10-08: From the user: joins render merged, and snap-dependent. Runs attached to each other, or to the same lift station or building, draw as one shape: the renderer shades each pixel's trail fill once (a stencil pass, `drawTrailLayers`), each run leaves out its edges that fall inside a run it's joined to (`joinedWith`, `runCovers`), and attached ends get rounded caps, so a lift top with several runs reads as one rounded pad. Runs that only overlap keep both outlines crossing, and a loose end gets an orange bar across it, so a missed snap shows. Checked on a test layout at Lift 1: two runs from its top merge into one pad, a branch attached to a run merges at the junction, and a loose run crossing them shows its outlines and orange ends.

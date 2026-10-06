---
title: Ground Materials
kind: plan
status: in progress
---

# Ground Materials

What the ground is made of, as data: rock (in a few kinds), scree, dirt, and meadow on the 1.25 m detail lattice, shared by the renderer and the sim. Snow sheds off steep faces and piles up below them, and only settles on ground that can hold it. Kirkwood's cliffs are the first test. This is step 4 of [[Terrain Realism]] ("Kirkwood's cliffs"), replacing the old plan's shader-only rock mask. Listed in [[Next Steps]].

## Why

Investigated 2026-10-06 on the re-imported [[Kirkwood]]: the headwalls render as smooth white snow with a few dark slivers, and the rock pinnacles are snowed over.

- [[Snow]] is one depth per 5 m cell, and cells steeper than 45° still hold a median of 11 cm. The shader draws 5 cm as fully white.
- The drawn snow is each corner's average over its four cells, and it raises the surface. A bare 68° cell next to a 1.5 m drift gets about 0.75 m at its corners, which pillows cliff lips and fills cliff bases, so faces look rounded.
- Exposed rock is one flat grey from slope, with no strata, cracks, or variety.
- The 1.25 m lidar already has the real cliff shape. Cover is the problem, not geometry.

Simulation over shader tricks: snow should really leave the faces, so the sim agrees with the picture (no skiing 11 cm on a 68° face).

## How it works

- **The material map** (`world.TerrainMaterial`) holds one material per sample of the 1.25 m detail lattice, saved with the world in scenarios and player games. A map without one draws as before.
- **Materials**: meadow (the old height-driven ground), dirt, scree, and rock. Auto material only sets rock and meadow; dirt and scree are there for a brush and palettes later.
- **Auto material** is a world layer ([[Terrain Layers]]) with no strength slider, first, before Auto trees and Auto snow (`internal/scene/materialgen.go`). Rock is generous: everything steeper than 40° (±3° of 10 m noise, so edges are ragged), plus sharp convex lips up to 12° under that (negative Laplacian ≥ 0.3 per metre over 2.5 m). A 3 × 3 majority pass removes single-sample lidar speckle. Everything else is meadow. Maps without lidar get no material map ("needs lidar").
- Rerunning a world layer reruns the world layers after it, so trees follow a material change.
- **Trees** don't grow on rock or scree: Auto trees removes any that land there.
- **Rock rendering** (`terrain.frag`): one procedural rock wherever ground is rock, from the material map, or from slope on maps without one, modelled on Kirkwood's andesitic volcanic breccia (old mudflow debris). It's built in 3D world space, so near-vertical faces don't stretch: about 2 m boulders from cellular noise, dark andesite to pale grey with a few rusty ones, blending softly into a grey-brown matrix; vertical weathering streaks and grooves; fine grain that fades with distance; faint flow banding tens of metres apart; broad oxidised patches. The bump lighting comes from screen-space derivatives of the rock's relief, so it costs nothing extra. None of it is computed under 5 cm of snow or more.
- **Snow sheds** (`world.ShedSnow`, `snowshed.go`). Each cell holds at most a limit for its slope: everything to 35°, falling to about 0.3 m of water at 45°, 5 cm at 50°, 1 cm at 55°, and 5 mm past 60°. Rock in the material map holds only a 1 cm dusting whatever the slope, so a cell's limit is the slope's limit over its non-rock share plus the dusting over its rock (a gentle cell with some ground keeps whatever falls, on that ground). What a cell can't hold leaves its top layer first, then its base, and goes to its lower neighbours in proportion to how steeply the ground drops to each, landing in their base. Cells run highest first, so each has everything from above before it sheds; a cell with no lower neighbour keeps its snow. Snow is conserved. It runs at the end of Auto snow, after each day's snowfall in the sim (which lands evenly everywhere), and after the editor's Add Storm. The sort order and rock fractions are cached until the ground or material map changes: 11 ms a pass on Kirkwood, 0.15 s the first time.
- **Snow on rock is drawn from the same data**: the cell's snow lies on its ground and ledges, so rock points draw bare apart from a patchy dusting, and the tessellation doesn't raise rock by the snow depth, so lips stay sharp. Only with a material map; other maps draw snow as before.
- Non-rock ground blends the four material samples around each point. The "Ground" overlay shows the map whatever covers it: meadow green, dirt brown, scree pale grey, rock near-black.
- Kirkwood: Auto material takes 0.13 s, the save grows by 0.5 MB, and 2.6% of the map is rock: the headwalls and the outcrops that show dark in the lidar. A full-map frame costs about 0.8 ms more (12.5 → 13.2 ms), mostly reading the material map.

## Steps

1. Done: the material map and the Auto material layer, saved with the world, trees kept off rock and scree, and a "Ground" overlay. Simplified after trying it: no slider, one generous rock rule, one rock texture. Snow still covers the cliffs until steps 2–3.
2. Done: snow sheds from steep cells down the fall line and settles where the slope eases, in Auto snow, after daily snowfall, and after Add Storm. On its own it barely changed Kirkwood: Auto snow already sheds by cell slope, and the rock faces are steep at 1.25 m but only 30–45° over a 5 m cell.
3. Done: rock holds only a dusting, so rock-heavy cells shed, and the renderer draws snow on a cell's ground and ledges, not its rock, without raising rock by the snow depth. Kirkwood's 45–50° cells went from 0.33 m of snow to 0.01 m, and the headwall shows bare rock bands with snow in the gullies and on the ledges.
4. Done: one procedural rock modelled on Kirkwood's volcanic breccia (see How it works). Good enough for now; revisit once snow sheds and more rock shows, or when a scenario needs a different rock.
5. Sim behaviour from materials: avalanches start on loaded slopes above rock bands, guests avoid rock ([[Skiing]]) except experts dropping small cliffs (the "send it" easter egg).
6. Later: a material brush in the editor; a finer rock texture, perhaps a normal map, for close views; different rock textures for different rock types and terrains (Alta's granite and quartzite next to Kirkwood's volcanic breccia), chosen per scenario; and reshaping cliffs as geometry only if they still read as smooth ramps after steps 1–4.

## Open questions

- Maps without lidar have no detail lattice and so no material map. Should they get one at the cell grid?
- Rock colours per scenario (Alta's pale granite) could tint the one rock texture rather than add rock materials.
- `VisualElevationAt` (where objects and agents stand) still adds the cell's full snow on rock points. Fine while nothing stands on rock; revisit with step 5.

## Log

- 2026-10-06: Planned after looking at Kirkwood's cliffs. The user chose sim-side shedding over a shader trick, and a general material map (rock kinds, dirt, scree) over a rock mask.
- 2026-10-06: Step 1: the material map and Auto material layer. Scree only below rock, a majority pass against lidar speckle, and rock kinds in wandering horizontal bands came from looking at Kirkwood with the Ground overlay. Existing scenarios get a material map by unticking and reticking Auto material, or by importing again.
- 2026-10-06: Simplified at the user's call: Auto material has no slider (it only moved a slope threshold, and at low settings turned cliff faces into dirt), rock is everything past 40° plus convex lips, and the auto layer no longer makes scree, dirt, or rock kinds. Built one procedural rock texture for step 4. Found and fixed the `-screenshot` camera target sitting at height 0, which had been framing every close-up hundreds of metres off.
- 2026-10-06: Rock texture pass for Kirkwood: replaced the regular layers and cracks (too banded) with volcanic breccia (boulders in a matrix, vertical streaks, faint flow banding), and moved the bump lighting to screen-space derivatives. A close view of the headwall went from 11.4 to 9.6 ms on the GPU, cheaper than the banded version.
- 2026-10-06: Steps 2–3. Shedding by cell slope alone moved almost nothing on Kirkwood, so rock now holds a dusting in the shed limit, and the renderer draws the cell's snow off its rock. Fixed infinity × 0 on all-rock gentle cells (NaN snow) before it shipped, and cached the shed's sort and rock fractions for the daily pass.
- 2026-10-06: User signed off on steps 2–3 ("looking fantastic"). Asked for later: a finer rock texture or a normal map, and different textures for different rock types and terrains (added to step 6).
- 2026-10-06: A water material for lakes levelled by the Lakes layer: trees stay off it, and scoured ice shows through thin snow ([[Creeks and Lakes]]).
- 2026-10-06: Water split into ice, thin ice, and open water, set daily by each lake's ice ([[Creeks and Lakes]]).

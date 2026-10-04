---
title: Terrain Realism
kind: plan
status: planned
---

# Terrain Realism

Make imported mountains look and behave like the real place. Boreal reads a little uniform and plastic, and [[Kirkwood]] more so: big smooth snow sheets with the same sheen everywhere, steep walls that look like steep snow, smooth valleys where the real ones are cut by creeks, and snow that ignores which way a slope faces. Covers [[Terrain]], [[Snow]], and [[Rendering]]. Listed in [[Next Steps]].

## What's there today

- The terrain is one heightfield on the 5 m grid, drawn as a single mesh with a vertex at every cell corner, shaded as snow or bare ground. Steep faces only differ by being bare. Objects sit on the mesh through `VisualElevationAt`.
- The editor's auto-snow ([[Scenario Editor]]) shapes depth by elevation, slope, curvature, drainage, and wind direction, but not by sun.
- The sim's melt already uses sun: degree-days scaled by direct sun for the date, slope, and aspect, with ridges casting shade (`world.HorizonMap`). But the sun path is fixed at 45°N for every map (`resortLatitudeDeg` in `sim/snowmelt.go`), and scenarios start with no snow, so a north face and a south face only diverge after weeks of play.
- No creeks or lakes. Kirkwood's lake at the north edge is just flat ground.
- Terrain tiles come in at zoom 14, about 7 m per pixel at Tahoe's latitude, so the import can't supply detail finer than the grid.

## Decisions

- **Climate goes in the scenario file as parameters.** Latitude (for the sun path), and the snow and weather numbers the generator and sim need, are stored per scenario. Where we have station data (SNOTEL in the US) we fit them; elsewhere we look them up or guess. No live data fetching in the game.
- **Cliffs: simplified placed meshes and greebles, with shaders doing most of the look.** Size is the constraint: Kirkwood is 600 × 600 cells, and a [[Scenario Campaign|Whistler Blackcomb]]-scale map will be several times that. Meshes should be few, low-poly, and instanced.
- **Water barely needs rendering.** What a creek really adds is terrain shape: a watershed has more detail than the 5 m grid holds. Get the shape right and pathing follows on its own, because skiers go down into a creek bed but don't like climbing out again.
- **Pond skimming is for later.** A special allowance for a water feature that guests normally path around (already in [[Next Steps]] as an idea).

## Steps

In this order:

1. **Cliffs.**
   - Find rock from slope and curvature: faces past about 45°, and the convex lips above them.
   - Shader first: a rock material with strata and cracks, blended in by slope, plus the bare-rock tone where snow can't stick.
   - Then a small set of simplified rock meshes and greebles, instanced along the steepest faces and lips so they don't read as smooth ramps. Cap the count per area and drop them at a distance.
   - Behavior: snow doesn't stick to rock, avalanches start above it ([[Avalanche]]), and guests avoid it ([[Skiing]]), except experts who drop small ones (the "send it" easter egg).
2. **Finer terrain where it matters.** An adaptive terrain mesh: the 5 m grid by default, refined in areas flagged as detailed (cliffs, creek beds, sharp ridges).
   - The sim grid stays the source of truth for snow, pathing, and building. Refined areas add render detail on top.
   - Detail is procedural, shaped by what the grid already knows: flow direction for creek channels, slope for rock bands. Where a finer elevation source exists (USGS 3DEP lidar in the US), it could be used instead.
   - Splitting the mesh into chunks also gives distance culling and level of detail, which large maps need anyway.
   - Seams between levels must not crack, and `VisualElevationAt` has to read the refined surface so guests and objects don't float or sink. Keep the detail small enough that the grid's heights stay a fair stand-in for the sim.
3. **Creeks as terrain.** Trace streams from flow accumulation (the forest generator already computes it) and cut their beds through step 2's finer mesh: a narrow channel with banks, getting deeper and wider downstream. Pathing then handles them through slope alone. Water itself can be a simple tint, ice, or snow over the bed; no water rendering needed. Flat closed basins like Kirkwood's lake become frozen, snow-covered flats. Kirkwood's meadow creek is the first test.
4. **Snow that doesn't look plastic.** Sparkle (view-dependent glints that twinkle as the camera moves), fine wind texture and sastrugi on exposed snow, a soft blue tint in shade and in deep snow, different looks for powder, wind crust, ice, and slush ([[Snow]]), and tracks that break up groomed sheets. Where snow is thin, let rock, dirt, and grass show through in patches instead of a uniform fade.
5. **Auto-snow from the scenario's climate.**
   - Add climate parameters to the scenario data and save format ([[Save Format]]): latitude first, then snowpack and weather numbers. Melt reads the scenario's latitude instead of the fixed 45°N (Kirkwood is at 38.7°N).
   - The auto-snow generator adds a sun term using the same sun model and horizon map as melt: south faces and sunny spots lose snow, north faces and shaded gullies keep it.
   - Scenarios can start with a realistic snowpack for their start date, from the climate parameters, by elevation and aspect. The same parameters feed the per-scenario weather in [[Scenario Goals and Rules]].
   - Fill the parameters in for Boreal and Kirkwood from SNOTEL, and guess for scenarios outside the US until better data turns up.
   - The goal for Kirkwood: north and south faces look and ski very differently, so a spot that's usually bare in reality, like one near the parking lot that looks inviting for a lift today, is bare in the game too.

## Open questions

- Which climate parameters to store beyond latitude: a snowline, a start-of-season depth curve, storm frequency, typical temperatures?
- How finely to refine (2.5 m, 1.25 m?), and how much of a large map can be refined before memory and frame time suffer.
- Whether refined detail ever feeds back into the sim, for example a creek bed so narrow it's lost on the 5 m grid.

## Log

- 2026-10-04: Planned from the user's notes after building Kirkwood: cliffs, then creeks and water, then snow rendering, then auto-snow from real-world data.
- 2026-10-04: Decisions: climate goes in the scenario file as parameters, found or guessed outside the US. Cliffs get simplified instanced meshes and lean on shaders, with large maps in mind. Creeks matter as terrain shape, not rendered water, and pathing follows from slope. Added an adaptive mesh, finer around cliffs and creeks, as step 2. Pond skimming deferred.

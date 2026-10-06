---
title: Terrain Realism
kind: plan
status: planned
---

# Terrain Realism

Make imported mountains look and behave like the real place. Boreal reads a little uniform and plastic, and [[Kirkwood]] more so: big smooth snow sheets with the same sheen everywhere, steep walls that look like steep snow, smooth valleys where the real ones are cut by creeks, and snow that ignores which way a slope faces. Covers [[Terrain]], [[Snow]], and [[Rendering]]. Listed in [[Next Steps]].

## What's there today

- The terrain is one heightfield on the 5 m grid, drawn as chunked triangle patches with a control point at every cell corner, subdivided on the GPU by on-screen size, with an optional 1.25 m detail height texture on top (step 1). Shaded as snow or bare ground. Steep faces only differ by being bare. Objects sit on the mesh through `VisualElevationAt`.
- The editor's Auto tool snow ([[Scenario Editor]]) shapes depth by elevation, slope, curvature, drainage, and wind direction, but not by sun. The Auto snow terrain layer adds the climate, sun, and terrain shade ([[Terrain Layers]]).
- The sim's melt uses sun: degree-days scaled by direct sun for the date, slope, and aspect, with ridges casting shade (`world.HorizonMap`). The sun path follows the map's latitude for imported maps (45°N for drawn ones).
- No creeks or lakes. Kirkwood's lake at the north edge is just flat ground.
- Terrain tiles come in at zoom 14, about 7.5 m per pixel at Tahoe's latitude, so outside lidar coverage the import can't supply detail finer than the grid. Inside it, the import takes cells and 1.25 m detail from USGS 1 m lidar ([[Terrain Import]]).

## Decisions

- **Climate goes in the scenario file as parameters.** Latitude (for the sun path), and the snow and weather numbers the generator and sim need, are stored per scenario. Where we have station data (SNOTEL in the US) we fit them; elsewhere we look them up or guess. No live data fetching in the game.
- **Subdivide the mesh first; bake rock and creeks into it.** Cliff faces, rock bands, and creek beds become real geometry in a finer terrain mesh, not separate objects. Instanced rock meshes come only if the baked mesh can't carry the look. Size is the constraint: Kirkwood is 600 × 600 cells, and a [[Scenario Campaign|Whistler Blackcomb]]-scale map will be several times that.
- **Detail is pre-baked, not generated at runtime.** A pass analyzes the heightmap after import (slope, curvature, flow) and writes the refined detail into the scenario. Play just loads it. The editor reruns the pass when terrain changes.
- **Water barely needs rendering.** What a creek really adds is terrain shape: a watershed has more detail than the 5 m grid holds. Get the shape right and pathing follows on its own, because skiers go down into a creek bed but don't like climbing out again.
- **Detail is a height grid drawn on the GPU.** Every pass reads and writes a fine 1.25 m height grid plus masks; the renderer draws it through tessellation from a height texture, so rerunning a pass is a re-upload, not a mesh rebuild. A CPU mesh per chunk would only help with overhangs, which no pass makes. Near-vertical faces get their fine detail from rock shading (strata, cracks, side-projected) rather than geometry; 0.625 m only if close-ups look soft.
- **Real data first where it exists.** Use 1 m lidar (USGS 3DEP) as the fine grid where it covers a map, and procedural passes elsewhere.
- **Baked detail feeds the sim.** Cell elevations are recomputed from the fine grid after a bake, so the grid stays the sim's source of truth but carries the cliffs and creek beds.
- **Pond skimming is for later.** A special allowance for a water feature that guests normally path around (already in [[Next Steps]] as an idea).

## Steps

In this order:

1. **Mesh subdivision.** The 5 m grid stays the mesh's control points; the GPU's tessellation stage subdivides it and adds detail from a height texture.
   - Split the terrain into 32 × 32-cell chunks with shared, indexed vertices (the flat face normal is computed per triangle in the shader), and skip chunks off screen. Snow and instability updates re-upload only the chunks they touch.
   - Each triangle edge picks its subdivision (1, 2, or 4) from its size on screen; neighbours compute the same level for a shared edge, so seams can't crack.
   - A detail height texture at 1.25 m, sampled in grid coordinates so subdivided points land on its texels. `VisualElevationAt` reads the same offsets so guests and objects don't float or sink.
   - First test: all-zero detail looks identical to today; a debug bump pattern shows no cracks across chunks or level changes; zoomed-out GPU time drops well below today's 16 ms at Kirkwood.
2. **Smoothing tools.** Boreal's lidar leaves a clear strip where the highway runs, which is a bit annoying on the map.
   - Done: "Auto-smooth roads" at import rebuilds the ground along OpenStreetMap roads, banks included, with natural roughness added back ([[Terrain Import]]). The same corridor-and-fill approach can place lakes and rivers, and clear ground for real lifts, later.
   - Done: "Smooth ground" at import blurs away features narrower than about 8 m everywhere (cat tracks, run grading, tubing lanes softened), sparing faces steeper than 35–45°. Run grading and cat tracks are the player's job.
   - Done: these passes are terrain layers, each switchable in the editor's Layers panel after import, and the base import reopens to change the square ([[Terrain Layers]]).
   - Editor brushes to smooth and flatten the ground (and probably raise and lower), editing the cells and the 1.25 m detail together, with cells recomputed from the detail under the brush. Could reuse the import's fill: paint a mask, fill it from the ground around it.
3. **Auto-snow from the real-world data.**
   - Done: latitude, longitude, time zone, base altitude, and a monthly climate are in the save ([[Save Format]]), filled in at import from Open-Meteo and SNOTEL ([[Weather]]); the sun, melt, and weather use them.
   - Done: the Auto snow terrain layer ([[Terrain Layers]]) runs a typical season from the climate with the same sun model, melt, and horizon map as the sim. South faces and sunny spots lose snow, and north faces and shaded gullies keep it. Imported scenarios start on the opening day with that day's snowpack.
   - The same parameters feed the per-scenario weather in [[Scenario Goals and Rules]].
   - Outside the US the reanalysis stands in for SNOTEL.
   - The goal for Kirkwood: north and south faces look and ski very differently, so a spot that's usually bare in reality, like one near the parking lot that looks inviting for a lift today, is bare in the game too.
4. **Auto trees and auto snow, separately and on import.** Done: they're the Auto trees and Auto snow terrain layers, run after every import ([[Terrain Layers]]). Left: re-import Boreal from scratch; steps 2–4 are what it needs to get back on track.
5. **Detail pipeline.** Passes run in Go on a fine 1.25 m height grid, after import or from the editor's "Bake terrain detail", in this order: repair, erosion, cliffs, creeks.
   - The fine grid starts as a smooth (bicubic) upsample of the 5 m ground, or real 1 m lidar where it exists (USGS 3DEP; check Kirkwood first), then a repair pass fixes spikes, pits, and tile seams.
   - Saved in the scenario as offsets from the smooth base plus rock and creek masks.
   - Each 5 m cell's elevation is recomputed from the fine grid, so snow, pathing, and building see cliffs and creek beds through slope.
   - Built in the order cliffs, creeks, erosion, since cliffs and creeks are the visible win.
6. **Baked cliffs.** Kirkwood first, after Boreal is back on track. Replaced by [[Ground Materials]]: the lidar already has the cliff shape, and the problem is snow covering it. Materials plus snow that sheds come first, and geometry only if it's still needed. The original plan:
   - A detail pass finds rock from slope and curvature (faces past about 45°, and the convex lips above them) and bakes it into the refined mesh: steps, ledges, strata, and broken edges as real geometry, plus a rock mask for the shader.
   - The terrain shader draws rock from the mask: color, strata, and cracks, so the baked shape and the material line up.
   - Instanced rock meshes and greebles only if the baked mesh still reads as a smooth ramp up close. If needed, keep them few, low-poly, and dropped at a distance.
   - Behavior: snow doesn't stick to rock, avalanches start above it ([[Avalanche]]), and guests avoid it ([[Skiing]]), except experts who drop small ones (the "send it" easter egg).
7. **Baked creeks and lakes.** The same pass traces streams from flow accumulation (the forest generator already computes it) and cuts their beds into the refined mesh: a narrow channel with banks, getting deeper and wider downstream. Pathing then handles them through slope alone. Water itself can be a simple tint, ice, or snow over the bed; no water rendering needed. Flat closed basins like Kirkwood's lake become frozen, snow-covered flats. Kirkwood's meadow creek is the first test.
   - OpenStreetMap water says where they really are: lakes (`natural=water`) and rivers and streams (`waterway=*`), fetched at import like the lifts overlay. Caples Lake at Kirkwood might come along for free.
8. **Erosion.** Hydraulic erosion for gullies and fans, and thermal collapse for scree below cliffs. Runs before cliffs and creeks in the pipeline, built after them.
   - Done: "Erode" at import runs droplet hydraulic erosion on the lidar detail after smoothing: rills and small gullies down the fall line, a few metres deep at most ([[Terrain Import]]). Left: thermal collapse for scree, and erosion on terrain without lidar (needs the detail pipeline).
9. **Snow that doesn't look plastic.** Builds on [[Graphics Base]], which handles anti-aliasing, light balance, broad snow variation, and view-driven sparkle. This step adds fine wind texture and sastrugi on exposed snow, a soft blue tint in deep snow, different looks for powder, wind crust, ice, and slush ([[Snow]]), and tracks that break up groomed sheets. Where snow is thin, let rock, dirt, and grass show through in patches instead of a uniform fade.
## Open questions

- Climate beyond the monthly averages: a start-of-season depth curve (SNOTEL has snow depth), and whether wet days should split rain and snow by altitude instead of at the base.
- Whistler-scale maps: a dense 1.25 m grid is about 100 MB of 16-bit heights; may need sparse tiles or a coarser grid away from cliffs and creeks.
- A creek bed narrower than a cell is averaged away when cells are recomputed from the fine grid; does pathing need the creek mask too?

## Log

- 2026-10-04: Planned from the user's notes after building Kirkwood: cliffs, then creeks and water, then snow rendering, then auto-snow from real-world data.
- 2026-10-04: Decisions: climate goes in the scenario file as parameters, found or guessed outside the US. Cliffs get simplified instanced meshes and lean on shaders, with large maps in mind. Creeks matter as terrain shape, not rendered water, and pathing follows from slope. Added an adaptive mesh, finer around cliffs and creeks, as step 2. Pond skimming deferred.
- 2026-10-05: Sparkle, shade color, and broad snow variation moved to [[Graphics Base]], which comes first.
- 2026-10-05: Reordered: mesh subdivision first, then cliffs and creeks baked into the mesh by a detail pass after import, pre-baked into the scenario. Instanced rock meshes only if the baked mesh isn't enough.
- 2026-10-05: Approved the plan: a fine 1.25 m height grid drawn on the GPU through tessellation, a multi-pass detail pipeline (repair, erosion, cliffs, creeks) feeding the sim cells, lidar where it exists, Kirkwood cliffs first. Added erosion as its own step. Started step 1.
- 2026-10-05: Step 1 done. 32 × 32-cell chunks (362 at Kirkwood) with shared vertices; per-corner snow moved into two textures, so a snow change uploads about 6 MB instead of the whole 146 MB vertex buffer; per-edge subdivision 1–4 (8 with detail); `world.TerrainDetail` at 1.25 m drawn by the tessellation stage, read by `VisualElevationAt` and trees. Kirkwood GPU time: 16.1 → 6.5 ms zoomed out, 7.7 → 3.3 ms zoomed in; CPU render 7.8 → 0.9 ms. Fixed snow being drawn at double depth after any snow change, and `VisualElevationAt` now finds the jittered triangle. `-detail-test` shows no cracks across chunks or levels. 1 m lidar covers Kirkwood (USGS 3DEP projects CA_UpperSouthAmerican_Eldorado_2019 and CA_SierraNevada_B22, 10 km GeoTIFF tiles, UTM zones 10 and 11).
- 2026-10-05: Fetched Kirkwood lidar with `tools/lidar` (`internal/geo`: TNM search, HTTP range reads of the cloud-optimized GeoTIFFs, LZW plus float predictor). 272 internal tiles in 28 s. Aligned to the map by a shift and scale search: +1.0 m east, −9.5 m south, scale 0.998, misfit 4.45 → 3.50 m RMS. Wrote a 2397² heights file at 1.25 m with no gaps; `-detail-file` draws it. Open snow now shows real gullies, ribs, and benches; the ridge profile gets a ragged rock edge. GPU cost about +0.15 ms. Not yet saved into the scenario.
- 2026-10-05: Detail is now part of the save (`detail`: offsets in centimetres, delta-coded per row; Kirkwood's save goes from 2.8 to 8.2 MB, largest offset 45 m). `tools/lidar -write-save` bakes it into a copy; the preview is `kirkwood-lidar` in the saves folder. The bundled scenario is unchanged.
- 2026-10-05: Moved lidar into the editor's terrain import instead of patching saves: cells are averaged from the lidar too, gaps fall back to the tiles with a feathered seam, and the import's bounds are saved. Kirkwood's footprint: 100% coverage, 70 s, largest detail offset 24 m (45 m when the cells came from the tiles). The plan is to rebuild Kirkwood from a fresh import.
- 2026-10-05: The save now carries base altitude, time zone, and climate; imports fill them in. The sun follows the map's latitude, the clock is local time (Kirkwood's December sunrise 7:15, sunset 16:42), the weather chain draws from the climate (Kirkwood's January base averages −2.4 °C, not the generic −10 °C), thirst counts real altitude, and the editor's wind and snowline sliders start from the climate.
- 2026-10-05: Reordered from the user's notes after re-importing Boreal: smoothing tools (with OpenStreetMap roads for auto-smoothing), auto-snow from the real-world data, and separate auto trees and auto snow run on import come first, to get Boreal back on track. Then Kirkwood's cliffs, and creeks and lakes placed from OpenStreetMap water.
- 2026-10-05: Auto-smooth roads at import. Each road gets a corridor (pavement plus banks, grown over steep banks, gaps and small islands closed), filled by push-pull and relaxation from the ground either side, plus noise matched to the surrounding roughness. A first try without the noise left a featureless band, and per-road feathers left steps between carriageways; both fixed. Boreal: I-80 and its interchange gone, 26 roads, about 8 s.
- 2026-10-05: "Smooth ground" and "Erode" at import, after the lidar showed Boreal's run grading and cat tracks. Smooth is three 2.5 m box blurs, faded out on steep rock. Erosion is droplet-based (one drop per 1.25 m sample, 48 steps each), run in parallel on 256-sample tiles in a checkerboard so it's repeatable. Boreal: mean change 12 cm, deepest cut 5 m, 0.1 s and 1.4 s; the slope overlay loses the speckled benches and keeps the real steep faces.
- 2026-10-06: Steps 3 and 4 mostly done as the Auto trees and Auto snow terrain layers ([[Terrain Layers]]): treeline from the climate, a season snowpack from the climate with sun and shade, and imports starting on the opening day.
- 2026-10-06: Cliffs (Priority step 4) moved to [[Ground Materials]]: a material map, snow shedding in the sim, and rock shading, after finding Kirkwood's cliffs buried under averaged 5 m snow.

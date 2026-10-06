---
title: Creeks and Lakes
kind: plan
status: planned
---

# Creeks and Lakes

Water at [[Kirkwood]]: the creeks through the meadow and the valley floor, and lakes, Caples Lake among them if it's on the map. Creeks matter as the shape of the ground (beds, banks, snow bridges), not as rendered water; lakes are frozen, snow-covered flats in winter. OpenStreetMap says where the real ones are. Step 5 of [[Terrain Realism]], listed in [[Next Steps]].

## What we know

- The import already fetches OpenStreetMap roads, lifts, runs, and the ski-area boundary ([[Terrain Import]]) and keeps them in the scenario's base, where the editor's OpenStreetMap overlay draws them ([[Scenario Editor]]). Water now comes the same way (step 1).
- Kirkwood has 1.25 m lidar, which may already show the creek beds; the cliffs turned out to be there all along, buried under snow ([[Ground Materials]]). Lidar over open water is often missing or noisy, so a lake's surface may need flattening.
- Terrain passes run as layers in the editor's Layers panel ([[Terrain Layers]]); a creek or lake pass would be a ground layer there (Terrain Layers step 6). The road layer's corridor-and-fill code can carve or flatten along a line or inside a polygon.
- The material map ([[Ground Materials]]) can mark water and creek beds, so trees stay off them and the renderer can draw ice, gravel, or open water where snow is thin.
- The forest generator already computes flow accumulation, which could trace creeks OpenStreetMap doesn't have.

## Steps

1. Done: the import fetches OpenStreetMap water with the roads and ski features (streams and rivers from `waterway=river|stream|brook|canal|ditch|drain`, lakes from `natural=water` and `landuse=reservoir`, multipolygons included), keeps it in the base (`streams`, `lakes` in `terrain_base`), and the OpenStreetMap overlay draws it in cyan: streams by kind (rivers 6 m, streams 2.5 m, ditches 1.5 m, paler when they dry up in summer), lake outlines, and name labels. Kirkwood's square has 6 streams (Kirkwood Creek, Emigrant Creek) and 3 lakes (Caples Lake, Emigrant Lake, and an unnamed pond).
2. Done: lakes first, since an outline and a flat level are easy to trust. A Lakes ground layer (last, after Erode, no slider) levels the ground inside each lake and pond to a low percentile (10th) of the heights inside its outline, since lidar over water is the surface plus noise; outlines fill by even-odd scanlines over every edge, so a multipolygon split across ways still works (`geo.FlattenLakes`, `lakes.go`). Auto material marks the levelled lake as water, which keeps trees off it. In winter it's flat snow, and where the snow is under about 0.4 m the wind scours broad patches down to grey-blue ice. Kirkwood: 44 ha of Caples Lake and 4 ha of Emigrant Lake in the map corners, and a 0.2 ha pond; the ground rebuild takes 8.6 s with every layer.
3. Done: lake ice from the climate and an estimated depth, not a fixed rule.
   - Depth: no open data has it (OpenStreetMap rarely, lidar sees only the surface, and HydroLAKES is a huge download that estimates it the same way). It's the land's slope at the shore (the mean over three cells around the lake) carried on under the water at half the steepness, by distance from shore, capped at 4 m plus 6 m per tenfold of area in hectares (`setUpLakes` in `materialgen.go`, `Terrain.LakeDepth`, saved in decimetres). Kirkwood: pond 1.5 m, Emigrant Lake 10 m, Caples Lake 18 m.
   - Ice (`world.Lake`, `sim/lakeice.go`): each lake keeps a running Frost (°C·days below freezing since autumn, less warm days before any ice) and Thaw. A spot freezes once Frost passes 2.5 °C·days per metre of its depth, jittered ±30% by noise for a ragged front, so the shallows skin over first and the ice creeps out over weeks. It thickens with the square root of the frost since then (Stefan's law, 0.6× under snow) and melts 5 mm per °C·day of Thaw, up to three times faster in the shallows, so spring opens a moat at the shore. Under 5 cm it's thin ice.
   - The surface shows spot by spot in the material map as ice, thin ice, or open water: open water takes no snow (it falls in), mirrors the sky, and glints on ripples; thin ice is dark with patchy snow; ice is the snowy, scoured look. Auto snow runs the season to the start date and keeps only the snow that fell on each spot since it froze; the sim steps each lake daily from the day's temperature.
   - Kirkwood from the Carson Pass climate, all lakes: skims at the shore in late November; 19% iced on 12 December (Caples mostly open on opening day), 46% on 28 December, all frozen by 10 January; 90% open again by 1 May.
4. Streams, later. OpenStreetMap stream lines run far up the mountainside, where no water flows in winter, so they can't be carved as they are. Use them together with slope, flow accumulation, and our own erosion to decide where water really runs, then cut beds, mark creek-bed material, and let snow bridge small creeks and leave larger ones open.

## Open questions

- Streams: is it a shape problem (beds missing or smoothed away by Smooth ground and Erode) or a look problem (creeks buried under even snow)? And where along an OpenStreetMap line does a creek really start flowing?
- Lakes: should guests and pathing treat a frozen lake as walkable flat ground, or off limits? Today it's just flat ground.
- Do guests and pathing need to know about creeks (crossings, bridges), or is the ground's slope enough?
- Where should creeks come from where OpenStreetMap has none: flow accumulation, or nowhere?

## Log

- 2026-10-06: Planned as priority 0 after Ground Materials steps 1–4, from Terrain Realism step 5 (creeks and lakes, OpenStreetMap water).
- 2026-10-06: Step 1: OpenStreetMap water fetched at import, saved in the base, and drawn in the editor's OpenStreetMap overlay. Existing scenarios need importing again (Change..., then Import) to get it.
- 2026-10-06: Step 2: lakes and ponds levelled by a Lakes ground layer, marked as water, frozen and snowy with scoured ice. The user moved streams later: OpenStreetMap creek lines climb far up the mountainside, so streams need slope and erosion data too.
- 2026-10-06: Step 3: lake ice inferred from the climate (freeze-up delayed by size, Stefan growth, degree-day melt), shown as open water, thin ice, or ice. Caples Lake is open on opening day and frozen by late December, matching what the user knew of the real lake.
- 2026-10-06: Lake ice reworked after the user saw a lake go from open to frozen in about three days: an estimated depth per cell (shore slope × distance from shore, capped by size), and per-spot freezing by depth, so the ice grows from the shore over weeks and opens from the shore in spring.

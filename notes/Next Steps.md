---
title: Next Steps
kind: plan
status: planned
---
# Next Steps

Everything planned, in one place. A step big enough to need its own design gets a plan note in `notes/next/` and a link here; small items stay as a line under their area. When something ships, delete its line and update the system card's log.

Each item is a **bold name**, then what it is. *Needs* lists what has to exist first; *For* names the scenarios that want it, so the [[Scenario Campaign]] doubles as a rough order. To prioritize something, move it up into Priority.

## Priority

In the order to work on them:

1. [[Snow Tastes]]: steps 1–2 done (tastes rolled around five archetypes; snow underfoot moves mood and brings thoughts while skiing); next is step 3, great runs scored by taste, which is what lets snow quality reach the rating. Snow underfoot moves the mood target, each run ends in an event, guests choose lifts and trails by taste, and repeated laps get boring.
2. **Fix how services work**: the investigate-and-report items left under Bugs and Guests. Patrol ([[Patrol Day]]), falls ([[Lift Unloading]]), grooming (works: one cat grooms Boreal's run fully each night), and thirst and exhaustion (mostly how thoughts were counted, then the time scale) are done. Left: snow compacting too fast, and rechecking thirst at altitude and the after-closing tail with the new clock.

Then: **gameplay before scenario goals**. A headless three-lift Boreal (2026-10-06) runs about 70 guests a day with the rating stuck near 30%, so goals can't be set until guests can have a good day. Priority 2 covers the investigate-and-report items; then the guest pool per scenario ([[First Week Balance]]). [[Scenario Goals and Rules]] steps 1–4 shipped and are tabled until then.

Not ranked yet: everything below.

## Bugs

Diagnose each first, report the cause, then fix:

- **Thirst drains fast at altitude**: diagnosed 2026-10-07. Thirst drains only while skiing, at a base of five clock hours to empty × an altitude factor (+0.05% per metre: ×2.1 at Boreal's 2,200 m) × exertion (up to ×3 for beginners off groomed snow). With the old clock most guests (66%) left thirsty; with the new clock and food-court drinks it's 2%, so this may be fine now. Recheck the altitude factor (perhaps relative to the climate's altitude) if thirst still dominates at high resorts.
- **Fresh corduroy turns to crust overnight**: noticed 2026-10-07. Cats finish grooming by about 21:00, and the midnight weather update turns packed powder into crust on a cold clear day (`kindTransition`), so groomed runs open as crust. Decide whether grooming should come after the update, or packed powder shouldn't crust in one night ([[Snow]], [[Grooming]]).
- **Snow compacts far too fast**: found 2026-10-07 on the re-dressed Boreal. Fresh snow starts at a believable depth (150 mm of water as 83 cm on the opening storm), but within three days 135 mm sits in 23 cm (density about 0.6), and by mid-January 263 mm in 37 cm (about 0.7). Settled early-season snow is about 0.25–0.35, so the slopes look thin over a decent pack. Diagnose the settling in [[Snow]] (`SnowLayer` densification) before changing it.

## Scenarios and campaign ([[Scenarios]], [[Scenario Campaign]])

- [[Scenario Goals and Rules]], tabled until gameplay improves. Done: goal and rule data with progress saved, the rating saved, the daily check with win and lose, the goals panel with win and lose panels, and the editor's Goals tab. Left, in order: Boreal's goals (decided: three lifts open; a best day of guests in the thousands; a 7-day 70% rating streak; bonus: all within season one; no deadline; numbers to confirm with the headless three-lift run once guests can be satisfied), the no-grooming rule (*for* [[Asahidake]]), and unlocking in campaign order.
- **Campaign scenarios**: build the rest of the [[Scenario Campaign]] in order, each pulling in the features on its note's Needs list. [[Kirkwood]] exists and still needs [[Terrain Realism]], skill-matched goals, and hand-drawn parcels.
- **Rating per skill level**: separate ratings from beginners, intermediates, and experts, so a resort can't please one group and ignore another. *Needs* skill wants (under Guests). *For* [[Kirkwood]].
- **Locals score**: a goodwill meter for regulars that crowds, price rises, and heavy building lower. *Needs* guests remembering their visits. *For* [[Mad River Glen]].
- **Two base areas**: separate parking, lodges, and possibly ratings for each base on one map. *For* [[Palisades Tahoe]].

## Lifts ([[Lifts]])

- [[Lift Operations]]: refuse overlapping lifts, then breakdowns and a maintenance contract, wind holds that spare the gondola, and guests riding down. Wind holds *need* wind that varies by day. Breakdowns are *for* the Turnaround idea in [[Scenario Campaign]].
- **Surface lifts**: magic carpet first, then T-bar and rope tow, for beginner areas. None are lift types yet (today: double, quad, high-speed quad, 6-pack, gondola, heli). Kirkwood has two magic carpets and a T-bar in OpenStreetMap. *For* [[Kirkwood]], [[Boreal]], and Portillo in [[Scenario Campaign]].
- **Long gondola spans**: few, tall towers over terrain a chair can't cross ([[Vision]]). *For* [[Palisades Tahoe]], [[Zermatt]].
- **Riders on both sides of the line**: arriving riders spawn on both sides of the lift line, not one.
- **Partly filled chairs**: chairs that don't always fill, more often with beginners in line.
- **Lines around buildings**: lift lines wrap around buildings instead of through them ([[Pathfinding]]).
- **Lift attendants**: see Staff.

## Terrain, trails, and land ([[Terrain]], [[Trails]], [[Trees]], [[Parcels]])

- [[Terrain Realism]]: make imported mountains look and behave like the real place. Done: mesh subdivision, lidar import, the climate block, [[Terrain Layers]] (Boreal and Kirkwood re-imported with every layer), auto snow and trees from real data, and Kirkwood's cliffs ([[Ground Materials]]). Creeks and lakes are priority 0. Left, in order: editor brushes to smooth and flatten the ground (the road, smoothing, and erosion layers are done), thermal erosion for scree, and snow that doesn't look plastic.
- [[Creeks and Lakes]] leftovers: tune creek channels and how much is snow-bridged by eye on Kirkwood and Boreal; creeks freezing over in hard cold (from the lake model's frost); guests and pathing treating open creeks and open lakes as obstacles (and deciding whether a frozen lake is walkable); a lighter colour over shallow lake water.
- [[Ground Materials]] step 5: sim behaviour from materials. Avalanches start on loaded slopes above rock bands ([[Avalanche]]), and guests avoid rock ([[Skiing]]) except experts dropping small cliffs (the "send it" easter egg). Also where objects and guests stand on rock (`VisualElevationAt` still counts snow there).
- [[Land and Boundaries]]: a ski area boundary, land purchase as a real decision, protected land, and protected buildings. Hand-drawn parcels are *for* [[Kirkwood]]; expensive land is *for* [[Palisades Tahoe]].
- [[Gridless Drawing]]: draw parking lots, trails, and buildings as shapes instead of painting cells, keeping the grid only underneath for the sim and navigation. An idea for now: first work out what the grid buys each system.
- **Trail closures and slow zones**: the player closes a run or paints a slow zone; guests respect them, mostly. *For* [[Alta]].
- **Night skiing**: light chosen trails and lifts, pay to run the lights, and stay open past dark for an evening crowd. Operating hours and the day-night cycle already exist; trail lights need a lighting approach that scales past the 16 spotlights the shader handles today ([[Rendering]]). Lit evenings also cut into the time cats have to groom ([[Grooming]]). *For* [[Boreal]], which really does run at night.
- **Backcountry**: gates through the boundary to terrain with no patrol or grooming, for experts only. *Needs* the boundary from [[Land and Boundaries]].
- **Cat trails**: easy, narrow ways down for beginners that get crowded.
- **Cat skiing**: snowcats carry advanced guests to ungroomed terrain. *For* the Revelstoke idea in [[Scenario Campaign]].
- **Guests react to trunks**: glade-loving and tree-shy guests respond to trunks nearby instead of the cell's tree cover ([[Stored Trees]]). The trees taste in [[Snow Tastes]] reads this once it exists. *For* [[Asahidake]].
- **Editor glade tools**: a glade highlight and thinning slider in the [[Scenario Editor]], matching the play tool.
- **Real-world features from OpenStreetMap**: pick real lifts, roads, and parking structures and build them into the scenario. The editor's OpenStreetMap overlay already draws the lifts, runs, and roads on the ground with labels ([[Scenario Editor]]); left are parking and buildings (not fetched yet) and picking a feature to build from it.
- **Biomes**: forested, sub-alpine, and alpine zones changing build cost, grooming quality, and injury risk.

## Snow, weather, and avalanches ([[Snow]], [[Weather]], [[Avalanche]], [[Calendar]])

- **Per-scenario weather**: imported scenarios already roll daily weather from their own monthly climate ([[Weather]]). Left: climate for hand-drawn scenarios and places outside the US, and rain or snow chosen by altitude rather than at the base. *For* [[Killington]], [[Asahidake]].
- **Daily wind**: a wind direction and strength each day instead of one per scenario. Wind holds in [[Lift Operations]] *need* it.
- **Weather and guests**: weather changes arrivals and guest mood ([[Demand]], [[Satisfaction]]).
- **Drifts on lee slopes**: wind moves snow from windward faces into lee slopes and gullies. *Needs* daily wind.
- **Glaciers**: year-round snow at the top of high resorts. *For* [[Zermatt]].
- **More fast-forward targets**: skip to the first freezing night, first snowfall, or a base depth. *For* [[Killington]].
- **Avalanche control**: explosives, closures, and barriers. *For* [[Alta]].
- **Avalanche risk overlay**: show where slopes are loaded before anything releases. *For* [[Alta]].

## Safety ([[Ski Patrol]])

- **Patrol enforces slow zones**: patrollers stand at slow zones and slow fast skiers. *Needs* slow zones.
- **Clinic**: treats injuries on site instead of sending guests home.
- **Medevac**: a helicopter for serious incidents.

## Guests ([[GOAP]], [[Satisfaction]], [[Guest Types]])

- **Thirst and hunger repeat too often**: in the three-lift test, about 35 "I need something to drink" and 10 "I could really use a meal" thoughts per guest visit, with a bar and food court at the base ([[Satisfaction]]). Diagnose: guests not reaching the bar, needs falling too fast, or the thought repeating every tick. Found 2026-10-07: condition thoughts were re-added every 12 sim seconds while the condition held. Counted once a stretch ([[Satisfaction Rework]]), Boreal shows 0.7 thirsty thoughts a visit, and 17% of guests leave thirsty because it has no bar, which is what the thought should say. Left: check thirst drain against a bar once one exists.
- **Everyone leaves exhausted**: almost every guest left on "I'm too tired to ski" (−0.15), so the day ends on a bad note even for a good visit. Diagnose whether energy drains too fast, or whether a normal end of day should leave on a neutral or happy thought instead. Found 2026-10-07: the exit chart showed each guest's last thought, not why they left. With departure reasons ([[Satisfaction Rework]]), under 1% of Boreal's guests leave tired; most leave at closing. Left: guests still on the mountain two to three hours after the lifts close. Partly fixed 2026-10-07: closing used to zero everyone's patience, which made a rest outrank going home, so some guests stayed all night; now `World.ClosedForDay` makes `GoHome` outrank everything, and the last guest leaves 2.4–3.7 clock hours after closing. That tail was mostly the time scale, since fixed (a clock hour is now 900 sim seconds, not 180); recheck it.
- **Falls**: about 3 falls and 3 injuries per guest visit on plain green, blue, and black trails ([[Skiing]]). Diagnosed 2026-10-07 on the user's Boreal save. Every fall, 392 in two days, was a beginner 5–20 m from the lift's top post, within 15 s of unloading, on ground of about 20°. Balance went from full to zero in under 2 s, and almost all of the drain was the slope term. No intermediate or advanced guest fell. The run itself is gentle (2% of its cells over 15°). The cause is the top station's apron ([[Lifts]], `scene/lift_apron.go`, added 2026-10-06 in 049eb95). The apron is levelled to the natural ground 4 m beyond the post, which here is uphill, so it sits 2–4 m above the station's ground. `apronWeight` is 0 on the cable side of the post with no bank, so there's a 2–4 m drop within one 5 m cell (18–30°, and 29° at the top cell), right where guests ski off down the lift line. Beginners have a 10° comfort slope and lose balance at 0.8/s on 20°, so they fall almost every lap. Hard to find because the thoughts named the trail, not the spot; the trail's overall steepness was fine; the falls look like ordinary run falls a second after unloading; and the step only shows at cell resolution around the station. Separately, the balance model gives beginners no way through a short steep patch (no slowing or side-slipping), so any bump of about twice their comfort slope knocks them over in under 2 s. Fixed by [[Lift Unloading]] (2026-10-07): on the regraded save the only falls are getting off the lift, about 3% of rides. Left open: whether beginners should side-slip or slow on short steep patches elsewhere.
- **Guest pool per scenario**: done through [[Transit]]: each road entry has its own guest pool, so a scenario's catchment is the sum of its entries' pools. Boreal has two entries totalling 100,000 guests, enough for about 3,000 a day at a perfect rating.
- **What each skill wants**: the terrain half is in [[Snow Tastes]]. Beginners want rentals and easy terrain; intermediates want terrain plus food and places to rest; advanced skiers want terrain and no crowds. Feeds [[Demand]] and [[Satisfaction]]. *For* [[Kirkwood]].
- **Snowboarders**: guests already roll Snowboard but still ski and look like skiers.
- **Children and families**: their own guest type, arriving and moving as a group.
- **Guest goals beyond lapping**: find the shortest line, go to après-ski, stay near the lodge. Powder hunting moved to [[Snow Tastes]].
- **Regulars**: guests who remember their last visit and come back, or don't. *For* [[Mad River Glen]].
- **Mood can't climb past about 0.5**: on the regraded Boreal save (2026-10-07, 5 days) falls are gone and guests have 1.5 great runs a visit, yet the rating is 0.34–0.41. Satisfaction drifts toward a 0.5 baseline with about a one-hour time constant ([[Satisfaction]], `tickMood`), so a good event (+0.03 to +0.06) fades within the hour. A guest ends the day near 0.5 minus their active pulls (sick of waiting, thirsty), whatever kind of day they had. A 70% rating goal can't be reached until good experiences can lift the target itself, as in RollerCoaster Tycoon, where rides raise a guest's happiness target. Built as [[Mood Baseline]] (2026-10-07); the rating is still held near 0.4 by Closing zeroes patience (Bugs).
- **Rest loop**: on Boreal with one lift, guests rested about six times a visit (2.2 once falls were fixed, since falls drain energy); line waits drain patience faster than skiing restores it. Check the patience rates against lift line waits ([[Patience]]).
- **Crowding**: guests notice crowded lodges, not only lift lines; crowded runs are in [[Snow Tastes]]. *For* [[Mad River Glen]].
- **Mogul lovers**: an expert bombing a mogul run entertaining the lift above. Guests who seek moguls are the Bump Skier in [[Snow Tastes]].
- **Non-skiing guests**: come for attractions, food, and the village. *Needs* attractions (under Real estate and attractions).
- **Named complaints**: rating feedback that lists the top complaints (long lines, wrong difficulty, falls, full parking).
- **Guest trip history**: a gameplay version of the follow-guest panel with runs taken, vertical, and time on the mountain.

## Base area ([[Amenities]], [[Lodge Shell]], [[Pathfinding]])

- [[Building Interiors]]: a legend for the cutaway's colors and doors, then procedurally placed furniture.
- [[Rental Shop]]: the next amenity and the first staffing puzzle. Beginners want it (see skill wants).
- **Amenity quality and views**: better buildings attract more guests ([[GOAP]]) and can charge more.
- **Door queues and service rates**: buildings serve guests at a rate, and lines form at the door.
- **Lockers and ski school**: more base services; ski school *needs* staff.
- **Footpaths**: painted paths between buildings, with guests walking skis-off. *For* [[Zermatt]].
- **Ski racks**: where footpaths meet the snow. *Needs* footpaths.
- [[Building Tool]]: build a shell first (Lodge, Tent, or Shed, dragged out as a ghost), then assign rooms in the building's panel; services leave the toolbar. Planned after the user found the mixed menu confusing.
- **Lodge storeys and styles**: more storeys, a style choice, and a shuffle button.

## Staff

- **Employees as people**: they drive in from the map's road entries as carloads, like guests ([[Transit]]), park in an employee lot (a lot set aside for staff), and walk to their stations.
- **Employee commutes**: each entry has a pool of workers as well as guests; a long or snowed-in drive makes staff late or harder to hire, and the morning arrival shares the road with guests. *Needs* employees as people.
- **Employee goals**: [[GOAP]] for employees (get to work, take breaks, go home). *Needs* employees as people.
- **Lift attendants**: two per lift, top and bottom, with a third speeding loading on bigger chairs. Each lift already pays for two a day, but none are on the map. *Needs* employees as people.
- **Staffing amenities**: staff for rental, food court, bar, tickets, patrol, and the snowcat garage, replacing the flat daily cost per tile ([[Building Services]]). *Needs* employees as people.
- **Employee housing**: so a resort can staff up where commuting is hard: staff housing as a building service ([[Building Services]], [[Rotated Buildings]]), with fewer cars on the road. *For* [[Zermatt]].

## Roads and arrivals ([[Parking and Roads]], [[Demand]])

- **Road closures**: a closed road means no arrivals that day. *For* [[Alta]].
- **Trains**: a second way to arrive, with no parking footprint. *For* [[Zermatt]].
- **Tunnels**: roads and paths through terrain.
- **Parking choice**: guests pick lots weighted by distance to the lifts. Part of [[Transit]] step 5.
- [[Lot Surfaces]]: asphalt, gravel, or dirt per lot, with its own cost, capacity, and look. Planned, split out of [[Transit]].
- **Better traffic**: merging, turning lanes, signals, and more than one entrance per lot, for resorts past a couple of thousand cars a morning. Part of [[Transit]] step 5.

## Economy ([[Finance]], [[Demand]])

- [[Money Feedback]]: a floating "-$600" when you spend, cost previews for every tool that costs money, and income and cost flashes on the cash display.
- [[First Week Balance]]: starting cash buys a parking lot, a short lift, and a cat shed; the guest pool scales with the resort; a week of revenue buys about one lift. *Needs* the ticket-window bug fixed.
- **Full cost rebalance**: build and operating costs across the board. *Needs* [[First Week Balance]].
- **Loans**: from the old `NEXT.md`; a credit line already exists, so scope what loans add first. *For* the Turnaround idea in [[Scenario Campaign]].

## Real estate and attractions ([[Finance]], [[Amenities]])

- **Condos**: a burst of income from sales, and owners who become regular guests.
- **Hotels**: ongoing revenue, and guests who stay across days. *For* Portillo in [[Scenario Campaign]].
- **Houses**: more income than condos, more land.
- **Zoning**: zone the base area for parking, hotels, condos, and retail.
- **Shops**: retail in the base area.
- **Attractions**: an ice rink, a sledding hill, snowmobile tours, and cross-country trails.

## Rendering and engine ([[Rendering]], [[Model Pipeline]])

- **More performance**, if play shows it's needed: after coarse terrain levels by zoom (Kirkwood's whole map 13.7 → 8.6 ms GPU, 12 → 1.5 ms CPU), the measured leftovers are trees 2.0 ms (a simple far-away tree mesh), anti-aliasing 2.5 ms (already a setting), and the terrain fragment shader 3.4 ms; the horizon map rebuild (3.9 s of CPU on Kirkwood) after placing a lift may hitch.
- [[Graphics Base]]: steps 1–6 shipped (anti-aliasing, light balance, snow breakup, trees, haze, map edge). Left: bough snow that lingers after a storm (needs a recent-snowfall value in the weather sim), gamma-correct lighting, and post-processing. The sim half of **Storm lag** below is also still unchecked.
- **Rock textures** ([[Ground Materials]] step 6): a finer rock texture, perhaps a normal map, for close views; and different textures for different rock types and terrains, chosen per scenario (Alta's granite and quartzite next to Kirkwood's volcanic breccia). A material brush in the editor goes with it.
- [[Hiding the Grid]]: smooth the parcel fence and painted overlays so the 5 m cells don't show. Step 3 (grooming follows the cat) shipped with [[Real Grooming]].
- **Groomed edge artifacts**: some artifacts still show along the edge of groomed areas after [[Real Grooming]]; capture close-ups with `-groom-now` and clean up the edge in `terrain.frag` ([[Grooming]]).
- **Grooming over tracks**: decide how a cat pass treats skier tracks already in the snow: wipe them under the swath, fade them, or keep cutting them through the corduroy in the shader as now ([[Grooming]], [[Real Grooming]]).
- **Environmental variety**: tree species, deciduous trees, dead snags, saplings, shrubs, and rocks. Rocks overlap with cliffs in [[Terrain Realism]].
- **Animation approach**: choose between procedural in the shader (as now), glTF skinned meshes, baked keyframes, or blended poses.
- **Storm lag**: lag was reported after a heavy storm at [[Boreal]]. The GPU half is fixed (fresh snow zoomed out: 10.6 → 7.2 ms), and snow changes no longer re-upload the whole terrain vertex buffer every frame ([[Terrain Realism]] step 1). The sim half is unchecked, because the tutorial save has no guests; play a busy game through a storm with `-cpuprofile` to see if deep powder or avalanche checks slow the sim ([[Debug Tools]]).
- **5,000 guests**: persistent buffers, culling the guest batch, lower-detail skier meshes. Big maps also lean on the terrain chunks from [[Terrain Realism]] step 1.

## Easter eggs

Things that happen on their own when conditions are right, not placed by the player:

- **Pond skimming**: a warm day, a flat runout, and a puddle; an expert in a great mood crosses it and a crowd gathers. *Needs* water features from [[Terrain Realism]].
- **Send it**: a pro finds a small cliff and either stomps it or yard-sales and needs [[Ski Patrol]]. *Needs* cliffs from [[Terrain Realism]].
- **Bootpacking**: a rested pro hikes above the top lift to an untouched face, and other pros follow. Pairs with backcountry.
- **Gaper Day**: on the last weekend of the season some guests turn up in retro outfits.
- **Yeti sighting**: very rare, in alpine terrain with low traffic and heavy snow.

## Log

- 2026-10-01: Created from `NEXT.md`, `MVP.md`, `DESIGN.md`, and the cards, then deleted those three files. Only Stored Trees is ranked.
- 2026-10-01: Added the Scenarios section.
- 2026-10-02: Merged the user's Google Keep list: added First Week Balance, a Staff section, skill wants, glaciers, climate data, backcountry, cliffs and water, and more activities.
- 2026-10-02: Shipped [[Stored Trees]] and moved its leftovers into Uplift and terrain.
- 2026-10-02: Shipped [[Scenario Metadata]]. Ranked the campaign: Goals and Rules, then the scenarios.
- 2026-10-03: Added [[Money Feedback]].
- 2026-10-03: Added the ticket-window bug seen while testing [[Kirkwood]].
- 2026-10-04: Added [[Terrain Realism]] as priority 3, folding in the cliffs, water, and snow sparkle lines.
- 2026-10-05: Added [[Land and Boundaries]], [[Lift Operations]] (replacing the wear and wind-hold lines), and real grooming in the shader.
- 2026-10-05: Made [[Terrain Realism]] the only priority; moved [[Scenario Goals and Rules]] (keeping its step order) and the [[Scenario Campaign]] scenarios down to Scenarios.
- 2026-10-05: Cleaned up: every item now has a bold name, a one-line description, and *Needs* and *For* tags where they help. Regrouped into Bugs, Scenarios, Lifts, Terrain, Snow and weather, Safety, Guests, Base area, Staff, Roads and arrivals, Economy, Real estate, Rendering, and Easter eggs. Pulled in items from the scenario Needs lists (locals score, two base areas, long gondola spans, regulars, crowding) and linked them to their scenarios.
- 2026-10-05: Dropped the snowmaking water budget; it didn't sound fun.
- 2026-10-05: Ranked [[Graphics Base]] first and Real grooming second (with [[Hiding the Grid]] step 3), ahead of [[Terrain Realism]]. Moved sparkle and blue shade from Terrain Realism step 4 into Graphics Base.
- 2026-10-05: [[Terrain Realism]] now starts with mesh subdivision, with cliffs and creeks pre-baked into the mesh.
- 2026-10-05: Shipped [[Graphics Base]] steps 1–6; its leftovers stay at priority 1.
- 2026-10-05: Fixed the GPU cost of fresh snow after a storm; added **Storm lag** to check the sim side.
- 2026-10-05: Shipped [[Real Grooming]]; [[Terrain Realism]] moves up to priority 2.
- 2026-10-05: Reordered [[Terrain Realism]] from the user's notes after re-importing Boreal: smoothing tools, auto-snow from real data, and separate auto trees and snow on import come first, then Kirkwood's cliffs and lakes. Added real-world features from OpenStreetMap (later), and magic carpets first among surface lifts.
- 2026-10-05: Added [[Terrain Layers]] as the next Terrain Realism step: the import's smoothing and erosion passes move into a Layers panel in the editor.
- 2026-10-06: [[Terrain Layers]] step 1 shipped: Layers panel in the editor, base saved in scenarios.
- 2026-10-06: Auto trees and Auto snow shipped as terrain layers, snow from the climate, imports starting on the opening day; Terrain Realism items 3 and 4 folded into item 1.
- 2026-10-06: Added layer strength sliders as priority 0, from the user's notes after trying the Auto layers.
- 2026-10-06: [[Ground Materials]] steps 1–4 shipped (material map, volcanic rock texture, snow that sheds off steep ground and rock). Priority cleared of finished work: Graphics Base, Terrain Realism, and Ground Materials leftovers moved to their sections, rock texture ideas added under Rendering, and [[Creeks and Lakes]] is the new priority 0.
- 2026-10-06: Lake ice from the climate and estimated depth shipped ([[Creeks and Lakes]]); a performance pass is the new priority 0 after the user saw lag zoomed in, with streams next.
- 2026-10-06: [[Creeks and Lakes]] mostly shipped (OpenStreetMap water, lakes and lake ice, stream tracing, meadows, creek channels); its leftovers moved under Terrain. Performance pass shipped its main fix; priority list cleared for the next pick.
- 2026-10-06: Fixed the ticket-window bug: the window worked all along; guests were blamed on it when a lift was stopped (lifts start stopped, by design) or had no trail for their level. See [[GOAP]], [[Demand]], [[Tickets]].
- 2026-10-06: Dropped the "Release cat is wrong" bug: a leftover note from the old `NEXT.md` that no one could reproduce or explain.
- 2026-10-06: [[Scenario Goals and Rules]] steps 1–4 shipped and tabled; gameplay comes first. Filed the headless three-lift findings as Bugs (patrol, grooming) and Guests items (thirst, exhaustion, falls, guest pool per scenario).
- 2026-10-06: [[Transit]] planned with the user and ranked next: simulated traffic, rectangular lots, gravel and dirt, editor entry points.
- 2026-10-06: [[Transit]] step 2 shipped: cars drive in from the entries, park, and drive home; Boreal got its two entries.
- 2026-10-06: [[Transit]] step 3 shipped: rectangular lots with a driveway to the nearest road; Boreal's lot redrawn.
- 2026-10-06: [[Transit]] steps 1–3 done and off the priority list; surfaces written up as [[Lot Surfaces]] and left unranked.
- 2026-10-06: Dropped the performance check from Priority; the user will play and re-add it if needed. Its measured leftovers moved under Rendering and engine.
- 2026-10-06: Ranked [[Rotated Buildings]], [[Building Services]], then fixing services, with the user.
- 2026-10-06: [[Rotated Buildings]] steps 1–2 shipped.
- 2026-10-06: [[Rotated Buildings]] step 3 shipped: shed, tent, and lodge, and lodge storeys.
- 2026-10-06: [[Building Services]] shipped (registry, patrol and garage as services); fixing how services work is now priority 2.
- 2026-10-06: Cross-linked staff (commutes, employee lots, housing) with [[Transit]] and [[Building Services]], and [[Lift Operations]] and [[Land and Boundaries]] with the building and service work.
- 2026-10-06: Diagnosed the patrol bug (snowmobiles parked on bare ground never move); planned [[Patrol Day]] with the user.
- 2026-10-06: [[Patrol Day]] steps 1–5 shipped. Noticed on the Boreal rig: guests still skiing two to three hours after the lifts close (may tie into the exhaustion item).
- 2026-10-06: Planned [[Building Tool]] (shell first, then rooms), unranked.
- 2026-10-06: [[Rotated Buildings]] done (Boreal rebuilt); fixing services is now priority 1.
- 2026-10-06: [[Patrol Day]]: patrollers hike to any injury; helivac and overtime noted for later.
- 2026-10-07: [[Satisfaction Rework]] planned with the user and ranked first; Fix how services work moves to second.
- 2026-10-07: [[Snow Tastes]] planned with the user and ranked second; folded in the terrain half of skill wants, mogul lovers, powder hunting, and crowded runs.
- 2026-10-07: [[Satisfaction Rework]] shipped; [[Snow Tastes]] is first. Thirst and exhaustion were mostly counting; falls are the main drag on the rating.
- 2026-10-07: Diagnosed falls: the top-station apron's unbanked cable side leaves a 2–4 m step where beginners ski off the lift.
- 2026-10-07: [[Lift Unloading]] planned with the user and ranked first, ahead of [[Snow Tastes]].
- 2026-10-07: [[Lift Unloading]] shipped; [[Snow Tastes]] is first again.
- 2026-10-07: Added Mood can't climb past about 0.5, found after the falls fix left Boreal's rating near 0.4.
- 2026-10-07: [[Mood Baseline]] planned with the user and ranked first.
- 2026-10-07: [[Mood Baseline]] shipped. Diagnosed Closing zeroes patience, which holds the rating near 0.4.
- 2026-10-07: Fixed closing zeroing patience (`World.ClosedForDay`); Boreal's rating went from about 0.4 to 0.46–0.49. Traffic patience noted on [[Patience]].
- 2026-10-07: [[Snow Tastes]] step 1 shipped.
- 2026-10-07: Diagnosed Boreal's thin early season: clear days 8–10 °C too warm, and a start date older than the snowpack it lays down.
- 2026-10-07: Fixed clear days running 15–20 °C too warm; Auto snow replays the season's weather; Boreal re-dressed at default snow strength, opening 20 December 2026. Found snow compacting too fast.
- 2026-10-07: Diagnosed a guest frozen swapping skis near the lodge.
- 2026-10-07: Fixed the guest frozen swapping skis (one live door for both checks, skis back on only past 40 m), rests at a lodge without a lounge head for the food court door, and closing time replans any plan that doesn't end in going home.
- 2026-10-07: Food courts pour drinks and meals fill thirst; thirst outweighs skiing below 0.25; corduroy is the read-only reason for a great run; per-guest arrival times from three hours before opening. Recorded the time scale, thirst drain, and starting satisfaction findings.
- 2026-10-07: A clock hour is 900 sim seconds (was 180). On Boreal: 27–30 guests lined up at opening, 7.7 great runs a visit, 2% leave thirsty, rating 0.52, about 42 s of CPU per day for about 80 guests. Queue limits became clock-based, and a guest facing only full lines now says so instead of blaming the ticket window.
- 2026-10-07: Guests start at their baseline (0.5), not 0.6. Closed the patrol and grooming bugs (patrol fixed by Patrol Day; on the user's Boreal save one cat grooms the run fully every night and skiing wears it to about 0.6 by mid-afternoon).
- 2026-10-07: [[Snow Tastes]] step 2 shipped. Noted groomed runs opening as crust.

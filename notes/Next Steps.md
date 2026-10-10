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

1. [[Demo]]: the free Steam demo ([[Release]]: demo, then early access, then guest types as DLC), the Boreal tutorial and Kirkwood, playable for a few hours, as its own stripped build. Steps in order: the magic carpet; terrain expectations (readouts, demand by terrain mix, a challenge preference); service staffing and quality ([[Service Quality]]); balance so the goals are reachable; goals and unlocking; the snow bugs; a Kirkwood map pass; a demo pop-up; UI polish (including a closed-resort sign); graphics polish; visible traffic; menus; the demo build.

Not ranked yet: everything below.

## Bugs

Diagnose each first, report the cause, then fix:

- **Fallen skiers walk too far uphill**: the user, 2026-10-08, watching the game. After a fall guests walk much further back uphill than is realistic. Suspects: in a yard sale the skis stay near where the fall began (up to 0.5 + 0.2 × speed metres along the line) while the body slides on, so the guest climbs back up the whole slide for them (`knockDown`, `collectSki` in `sim/falls.go`); or the walk back up toward a route point or carrot uphill after getting up. Diagnose which before changing ([[Skiing]]).
- **Fresh corduroy turns to crust overnight**: noticed 2026-10-07. Cats finish grooming by about 21:00, and the midnight weather update turns packed powder into crust on a cold clear day (`kindTransition`), so groomed runs open as crust. Decide whether grooming should come after the update, or packed powder shouldn't crust in one night ([[Snow]], [[Grooming]]).
- **Snow compacts far too fast**: found 2026-10-07 on the re-dressed Boreal. Fresh snow starts at a believable depth (150 mm of water as 83 cm on the opening storm), but within three days 135 mm sits in 23 cm (density about 0.6), and by mid-January 263 mm in 37 cm (about 0.7). Settled early-season snow is about 0.25–0.35, so the slopes look thin over a decent pack. Diagnose the settling in [[Snow]] (`SnowLayer` densification) before changing it. Also blocked moguls, which now gate on snow water instead of visible depth (2026-10-07).
- **Point-placed services fail silently with no free cell**: found 2026-10-07 fixing tests. `PlaceBuildingType` for a lodge, bar, or ticket office (tests, testbeds, the editor's ticket office) converts to a service building only on free cells (`placePointService`); with none (say, inside a parking lot) it quietly stays an old-style building with no tiles or doors that serves nothing, and guests looking for it go home. It should fail visibly or find room. Belongs with footpaths and pathing work ([[Pathfinding]], [[Lodge Shell]]).
- **Lift-station huts stand in the lift lines**: found 2026-10-08 measuring clearance round stations. The operator's hut in `lift_station.scad` sits 3.4 m back and 5 m out on the empty-chair side, which at a base station is inside the lift-line rows (rows stack downhill in 2.5 m bands, places step out in 2 m slots: the hut is row 2, slot 3), so guests are sent to places inside it: about 1,850 one-second samples a day within 1 m of base huts on Boreal (250 guests). Walkers now sidestep station parts, but not one at the place they're walking to. Decide where the hut should go (beside the load point uphill of the rows, or further out) in the model and `world.StationXZs` together ([[Lifts]]).
- **Novices get bored with one short green**: Boreal Goals Test, 2026-10-08, after the Sunset Blvd. fixes (falls 948 → 136, gave up 108 → 1 on the user's save). Novices (skill under 0.10) now only ride lifts whose terrain is all green, so on Boreal they keep to Lift4's Kiss A Bear, and with one short run they're soon bored: 83 bored departures a day against about 25. Partly gameplay (build them gentle terrain: the magic carpet, [[Demo]] step 1); check the boredom rate for a guest with one lift once there's a bunny area to compare with ([[Skiing]]).

## Scenarios and campaign ([[Scenarios]], [[Scenario Campaign]])

- [[Scenario Goals and Rules]], tabled until gameplay improves. Done: goal and rule data with progress saved, the rating saved, the daily check with win and lose, the goals panel with win and lose panels, and the editor's Goals tab. Left, in order: Boreal's goals (decided: three lifts open; a best day of guests in the thousands; a 7-day 70% rating streak; bonus: all within season one; no deadline; numbers to confirm with the headless three-lift run once guests can be satisfied), the no-grooming rule (*for* [[Asahidake]]), and unlocking in campaign order.
- **Campaign scenarios**: build the rest of the [[Scenario Campaign]] in order, each pulling in the features on its note's Needs list. [[Kirkwood]] exists and still needs [[Terrain Realism]], skill-matched goals, and hand-drawn parcels.
- **Rating per skill level**: separate ratings from beginners, intermediates, and experts, so a resort can't please one group and ignore another. *Needs* skill wants (under Guests). *For* [[Kirkwood]].
- **Locals score**: a goodwill meter for regulars that crowds, price rises, and heavy building lower. *Needs* guests remembering their visits. *For* [[Mad River Glen]].
- **Two base areas**: separate parking, lodges, and possibly ratings for each base on one map. *For* [[Palisades Tahoe]].

## Lifts ([[Lifts]])

- [[Lift Operations]]: refuse overlapping lifts, then breakdowns and a maintenance contract, wind holds that spare the gondola, and guests riding down. Wind holds *need* wind that varies by day. Breakdowns are *for* the Turnaround idea in [[Scenario Campaign]].
- **Surface lifts**: magic carpet first (in the [[Demo]]), then T-bar and rope tow, for beginner areas. None are lift types yet (today: double, quad, high-speed quad, 6-pack, gondola, heli). Kirkwood has two magic carpets and a T-bar in OpenStreetMap. *For* [[Kirkwood]], [[Boreal]], and Portillo in [[Scenario Campaign]].
- **Long gondola spans**: few, tall towers over terrain a chair can't cross ([[Vision]]). *For* [[Palisades Tahoe]], [[Zermatt]].
- **Riders on both sides of the line**: arriving riders spawn on both sides of the lift line, not one.
- **Partly filled chairs**: chairs that don't always fill, more often with beginners in line.
- **Lines around buildings**: lift lines wrap around buildings instead of through them ([[Pathfinding]]).
- **Lift attendants**: see Staff.

## Terrain, trails, and land ([[Terrain]], [[Trails]], [[Trees]], [[Parcels]])

- **Ropes and signs at trail splits**: the user, 2026-10-08. Where a cat track or long easy run meets or crosses a harder one, beginners can end up on it; real resorts rope off the line, fence the edge, or post "slow" and "experts only" signs. A player tool placing ropes, fences, and signs that skiers treat as obstacles or warnings, so the player shapes where guests go ([[Trails]]).
- **Trail steepness for the player** (held, the user 2026-10-08): a trail's steepest sustained pitch, a rating check ("steeper than a green should be in 2 places") marked on the map, and naming a fall's cause (too steep, too fast, trees). Guests never see the numbers; it's for the player.
- [[Trail Network]]: trails drawn as lines of nodes snapped to lifts, buildings, and other runs, each node with its own width; glades, bowls, and backcountry outlined as polygon zones. Steps 1–3 done (runs, the run tool, grooming clear of trees); left: areas (glades, bowls, backcountry) and redrawing Boreal's and Kirkwood's trails by hand, which have none until then.
- [[Terrain Realism]]: make imported mountains look and behave like the real place. Done: mesh subdivision, lidar import, the climate block, [[Terrain Layers]] (Boreal and Kirkwood re-imported with every layer), auto snow and trees from real data, and Kirkwood's cliffs ([[Ground Materials]]). Creeks and lakes are priority 0. Left, in order: editor brushes to smooth and flatten the ground (the road, smoothing, and erosion layers are done), thermal erosion for scree, and snow that doesn't look plastic.
- [[Creeks and Lakes]] leftovers: tune creek channels and how much is snow-bridged by eye on Kirkwood and Boreal; creeks freezing over in hard cold (from the lake model's frost); guests and pathing treating open creeks and open lakes as obstacles (and deciding whether a frozen lake is walkable); a lighter colour over shallow lake water.
- [[Ground Materials]] step 5: sim behaviour from materials. Avalanches start on loaded slopes above rock bands ([[Avalanche]]), and guests avoid rock ([[Skiing]]) except experts dropping small cliffs (the "send it" easter egg). Also where objects and guests stand on rock (`VisualElevationAt` still counts snow there).
- [[Land and Boundaries]]: a ski area boundary, land purchase as a real decision, protected land, and protected buildings. Hand-drawn parcels are *for* [[Kirkwood]]; expensive land is *for* [[Palisades Tahoe]].
- [[Gridless Drawing]]: draw parking lots, trails, and buildings as shapes instead of painting cells, keeping the grid only underneath for the sim and navigation. An idea for now: first work out what the grid buys each system.
- **Trail closures and slow zones**: the player closes a run or paints a slow zone; guests respect them, mostly. *For* [[Alta]].
- **Night skiing** ([[Night Skiing]]): light chosen trails and lifts, pay to run the lights, and stay open past dark for an evening crowd. Operating hours and the day-night cycle already exist; trail lights need a lighting approach that scales past the 16 spotlights the shader handles today ([[Rendering]]). Lit evenings also cut into the time cats have to groom ([[Grooming]]). *For* [[Boreal]], which really does run at night.
- **Backcountry**: gates through the boundary to terrain with no patrol or grooming, for experts only. *Needs* the boundary from [[Land and Boundaries]].
- **Cat trails**: easy, narrow ways down for beginners that get crowded.
- **Cat skiing**: snowcats carry advanced guests to ungroomed terrain. *For* the Revelstoke idea in [[Scenario Campaign]].
- **Guests react to trunks**: glade-loving and tree-shy guests respond to trunks nearby instead of the cell's tree cover ([[Stored Trees]]). The trees taste in [[Snow Tastes]] reads this once it exists. *For* [[Asahidake]].
- **Editor glade tools**: a glade highlight and thinning slider in the [[Scenario Editor]], matching the play tool.
- **Real-world features from OpenStreetMap**: pick real lifts, roads, and parking structures and build them into the scenario. The editor's OpenStreetMap overlay already draws the lifts, runs, and roads on the ground with labels ([[Scenario Editor]]); left are parking and buildings (not fetched yet) and picking a feature to build from it.
- **Biomes**: forested, sub-alpine, and alpine zones changing build cost, grooming quality, and injury risk.

## Snow, weather, and avalanches ([[Snow]], [[Weather]], [[Avalanche]], [[Calendar]])

- **Walkers leave tracks too**: the user, 2026-10-08. Guests walking (skis off, or shuffling on skis) should mark and pack powder like skiers do; today only skiing splats tracks (`splatSkierTrack`) and wears snow. Boot-packed holes in powder are a different mark from ski tracks.
- **Per-scenario weather**: imported scenarios already roll daily weather from their own monthly climate ([[Weather]]). Left: climate for hand-drawn scenarios and places outside the US, and rain or snow chosen by altitude rather than at the base. *For* [[Killington]], [[Asahidake]].
- **Daily wind**: a wind direction and strength each day instead of one per scenario. Wind holds in [[Lift Operations]] *need* it.
- **Weather and guests**: weather changes arrivals and guest mood ([[Demand]], [[Moments]]).
- **Drifts on lee slopes**: wind moves snow from windward faces into lee slopes and gullies. *Needs* daily wind.
- **Glaciers**: year-round snow at the top of high resorts. *For* [[Zermatt]].
- **Avalanche control**: explosives, closures, and barriers. *For* [[Alta]].
- **Avalanche risk overlay**: show where slopes are loaded before anything releases. *For* [[Alta]].
- [[Season Calendar]]: our own calendar, 10 days a month, open December to April, holidays on the 5th and 10th; a day in 80 s at the fastest speed and 320 s at normal. First measure how much skier movement a game hour can hold. To be ranked by the user.

## Safety ([[Ski Patrol]])

- **Patrol enforces slow zones**: patrollers stand at slow zones and slow fast skiers. *Needs* slow zones.
- **Clinic**: treats injuries on site instead of sending guests home.
- **Medevac**: a helicopter for serious incidents.
- **Skier-on-skier collisions**: today skiers only avoid each other (a 2.5 m danger zone when picking a line, and a swerve that clears a skier by 1.5 m); when that fails they pass through each other. A contact check like `hitsTrunk` would let two skiers collide: both fall, maybe an injury and a patrol call, more likely when fast, unskilled, or crowded. Makes crowding and mixed-skill runs visibly risky ([[Skiing]]). To be ranked by the user.

## Guests ([[GOAP]], [[Moments]], [[Guest Types]])

- **Calibrate the satisfaction ledger** (tabled by the user, 2026-10-07): every event amount and condition rate in `ai.Effects` was set for the old drifting model. On Boreal, intermediate and advanced guests carry "too easy" all day and can end at 0, and a storm day on an ungroomed green drops Cruisers from 0.53 to 0.32. Tune with a resort that has blue and black runs.
- **What each skill wants**: the terrain half is in [[Snow Tastes]]. Beginners want rentals and easy terrain; intermediates want terrain plus food and places to rest; advanced skiers want terrain and no crowds. Feeds [[Demand]] and [[Moments]]. *For* [[Kirkwood]].
- **Snowboarders**: guests already roll Snowboard but still ski and look like skiers.
- [[Groups]]: guests who come together ski together, led by one member; followers ski the leader's line with their own physics, tracks and collisions but no planning or steering fan, so a group costs about as much as two skiers ([[Crowd Scale]]). First step: a prototype measuring a follower's cost.
- **Children and families**: their own guest type, arriving and moving as a group. *Needs* [[Groups]].
- **Guest goals beyond lapping**: find the shortest line, go to après-ski, stay near the lodge. Powder hunting moved to [[Snow Tastes]].
- **Regulars**: guests who remember their last visit and come back, or don't. *For* [[Mad River Glen]].
- **Crowding**: guests notice crowded lodges, not only lift lines; crowded runs are in [[Snow Tastes]]. *For* [[Mad River Glen]].
- **Mogul lovers**: an expert bombing a mogul run entertaining the lift above. Guests who seek moguls are the Bump Skier in [[Snow Tastes]], and the moguls they seek are [[Moguls]].
- **Non-skiing guests**: come for attractions, food, and the village. *Needs* attractions (under Real estate and attractions).
- **Named complaints**: rating feedback that lists the top complaints (long lines, wrong difficulty, falls, full parking).
- [[Readable Complaints]]: review lines that name the place and why a need went unmet (full, missing, wrong kind, too far, too dear); today so far in the Reviews tab; occupancy in the building popup.
- **Guest trip history**: a gameplay version of the follow-guest panel with runs taken, vertical, and time on the mountain.

## Base area ([[Amenities]], [[Lodge Shell]], [[Pathfinding]])

- [[Building Interiors]]: a legend for the cutaway's colors and doors, then procedurally placed furniture.
- [[Services]]: the range of day services (snack stands, a café and the coffee need, ski school, retail, demo skis, lockers, guest services and a pass office, shuttles, activities), a card each; building anywhere, with cost and upkeep rising with elevation; supply by snowcat and lift later. Overnight services next.
- [[Rental Shop]] staff and ski racks: the shop is in ([[Service Improvements]]); staffing the morning rush and a rack at the snow are still story.
- [[Star Ratings]] and [[Scoreless Rating]]: each guest leaves 1–5 stars as levels (didn't ski, skied with a need unmet, every need met; 4★ and 5★ later), with no satisfaction score behind them: needs met set the star, unweighted good and bad moments explain it.
- [[Service Quality]]: formats at different price points (food court, restaurant, snack stand, shop), staff as a pool per room (headcount for timeliness, wage for quality through morale), hungrier guests with a lunch rush, and shopping.
- **Views**: better-sited buildings attract more guests ([[GOAP]]) and can charge more.
- **Lockers and ski school**: more base services; ski school *needs* staff.
- **Footpath follow-ups** ([[Pathfinding]]): paths that avoid building footprints. Footpaths, their editing, and their snow clearing shipped 2026-10-10.
- **Ski racks**: where footpaths meet the snow.
- [[Building Tool]] leftovers: presets (a one-tile ticket booth, a patrol shed with a garage), hints in the Services menu for what each service needs, and the open questions in its note (heated empty floor, rooms with no outside wall).
- **Lodge storeys and styles**: more storeys, a style choice, and a shuffle button.

## Staff

- **Employees as people**: they drive in from the map's road entries as carloads, like guests ([[Transit]]), park in an employee lot (a lot set aside for staff), and walk to their stations.
- **Employee commutes**: each entry has a pool of workers as well as guests; a long or snowed-in drive makes staff late or harder to hire, and the morning arrival shares the road with guests. *Needs* employees as people.
- **Employee goals**: [[GOAP]] for employees (get to work, take breaks, go home). *Needs* employees as people.
- **Lift staff as people**: the operators and line attendants stand at their posts ([[Lifts]]), but don't arrive or leave. *Needs* employees as people.
- [[Packed Snow]]: walkers slowed by deep or rough snow, feet packing it down, and lift sweepers keeping station areas clear.
- **Staffing amenities**: staff for rental, food court, bar, tickets, patrol, and the snowcat garage, replacing the flat daily cost per tile ([[Building Services]]). Staff pools per room come first in [[Service Quality]]; people fill them later. *Needs* employees as people.
- **Employee housing**: so a resort can staff up where commuting is hard: staff housing as a building service ([[Building Services]], [[Rotated Buildings]]), with fewer cars on the road. *For* [[Zermatt]].

## Roads and arrivals ([[Parking and Roads]], [[Demand]])

- **Road closures**: a closed road means no arrivals that day. *For* [[Alta]].
- **Trains**: a second way to arrive, with no parking footprint. *For* [[Zermatt]].
- **Tunnels**: roads and paths through terrain.
- **Parking choice**: guests pick lots weighted by distance to the lifts. Part of [[Transit]] step 5.
- [[Lot Surfaces]]: asphalt, gravel, or dirt per lot, with its own cost, capacity, and look. Planned, split out of [[Transit]].
- [[Large Vehicles]]: buses and RVs, which don't fit one stall: long-stall rows or paired stalls, bus drop-offs, and overnight RVs.
- **Better traffic**: merging, turning lanes, signals, and more than one entrance per lot, for resorts past a couple of thousand cars a morning. Part of [[Transit]] step 5.

## Economy ([[Finance]], [[Demand]])

- [[Money Feedback]]: a floating "-$600" when you spend, cost previews for every tool that costs money, and income and cost flashes on the cash display.
- [[Economy Balance]]: pacing in real minutes (something small every 1–3 days, a big project every week or two), payback days as the measure, a season economy report, then tuning running costs, build costs, and starting money. Open: how long a scenario should be, and whether the calendar changes.
- [[First Week Balance]]: starting cash buys a parking lot, a short lift, and a cat shed; the guest pool scales with the resort; a week of revenue buys about one lift. *Needs* the ticket-window bug fixed.
- **Full cost rebalance**: build and operating costs across the board. *Needs* [[First Week Balance]].
- **Loans**: from the old `NEXT.md`; a credit line already exists, so scope what loans add first. *For* the Turnaround idea in [[Scenario Campaign]].

## Real estate and attractions ([[Finance]], [[Amenities]])

- **Lodging** ([[Services]] Overnight): mixed, a trade-off between a [[Hotel]]'s ongoing revenue and selling land for [[Houses]], with [[Condos]] between; on-site beds raise demand, so a huge resort needs a village. Long term, after a playable demo.
- **Condos**: a burst of income from sales, and owners who become regular guests.
- **Hotels**: ongoing revenue, and guests who stay across days. *For* Portillo in [[Scenario Campaign]].
- **Houses**: more income than condos, more land.
- **Zoning**: zone the base area for parking, hotels, condos, and retail.
- **Shops**: retail in the base area.
- **Attractions**: an ice rink, a sledding hill, snowmobile tours, and cross-country trails.

## Rendering and engine ([[Rendering]], [[Model Pipeline]])

- [[Crowd Scale]]: measured 2026-10-08: a skiing guest costs ~3.3 µs per 1/30 s step, so 10,000 visitors a day would run at about 2× real time. The user wants one simpler sim, not coarse off-screen guests: a 1/10 s step is nearly free in fidelity (+44%), and the next costs are replanning and the walking pathfinder (see Bugs). To be ranked by the user.
- **Fast-forward leftovers** from [[Fast-Forward Performance]]: a few 40–90 ms sim frames around midday at 50× and up, cause not traced; snow tile uploads cost about 0.4 ms a frame at 50× (one call per tile; could batch); parallel steering now only starts for 64+ skiers per core (waking threads every 1/30 s step cost more than it saved); the sim on its own goroutine if turbo still drops frames; `-profile`'s synthetic resort gets no guests any more, so it measures nothing.
- **More performance**, if play shows it's needed: after coarse terrain levels by zoom (Kirkwood's whole map 13.7 → 8.6 ms GPU, 12 → 1.5 ms CPU), the measured leftovers are trees 2.0 ms (a simple far-away tree mesh), anti-aliasing 2.5 ms (already a setting), and the terrain fragment shader 3.4 ms; the horizon map rebuild (3.9 s of CPU on Kirkwood) after placing a lift may hitch.
- [[Graphics Base]]: steps 1–6 shipped (anti-aliasing, light balance, snow breakup, trees, haze, map edge). Left: bough snow that lingers after a storm (now its own plan, [[Bough Snow]], below), gamma-correct lighting, and post-processing. The sim half of **Storm lag** below is also still unchecked.
- [[Bough Snow]]: snow on the trees that builds through a storm and lingers after it, per cell instead of one value for the whole map: held for days in cold shade and up high, cleared first from low sunny slopes, shaken off by wind and washed off by rain. Today the trees go bare the day a storm ends. To be ranked by the user. *For* [[Asahidake]].
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
- 2026-10-07: Satisfaction is a ledger (events once, conditions per clock hour, recorded when the car leaves the map, with an end-of-day term), replacing [[Mood Baseline]]'s drift. Calibration later: on Boreal, intermediates and advanced guests on one green carry "too easy" all day and can reach 0.
- 2026-10-07: [[Snow Tastes]] step 3 shipped; calibrating the ledger tabled.
- 2026-10-07: [[Moguls]] planned with the user, ranked after [[Snow Tastes]].
- 2026-10-07: [[Snow Tastes]] step 4 shipped.
- 2026-10-07: [[Snow Tastes]] done (step 5: boredom sends guests home from a resort that's run out for them; crowds by taste). [[Moguls]] is first.
- 2026-10-07: Fix how services work is done: patrol, falls, grooming, thirst, exhaustion, and the after-closing tail (last guest now leaves 17–47 clock minutes after closing) are fixed or explained. Removed the resolved Bugs and Guests items; the snow compaction and overnight-crust bugs remain.
- 2026-10-07: Wrote up [[Service Improvements]] (unranked, pending the user's decisions).
- 2026-10-07: [[Moguls]] step 1 shipped.
- 2026-10-07: [[Moguls]] step 2 shipped.
- 2026-10-07: [[Moguls]] step 3 shipped.
- 2026-10-07: [[Moguls]] step 4 shipped.
- 2026-10-07: [[Moguls]] done (step 5, docs); off the Priority list, which is now empty.
- 2026-10-07: [[Moguls]] step 6 (slowing down and steering clear) prioritized by the user and shipped the same day; Priority is empty again.
- 2026-10-07: [[Service Improvements]] planned with the user (a needs model) and ranked first.
- 2026-10-07: [[Service Improvements]] step 1 shipped.
- 2026-10-07: [[Service Improvements]] step 2 shipped; door queues folded in.
- 2026-10-07: [[Service Improvements]] done (step 3: rolled needs, the rental shop, après, warming up). Ranked first: rental shops in the scenarios.
- 2026-10-07: Fixed the eight failing Go tests (stale fixtures and expectations); logged the silent point-placed service bug.
- 2026-10-07: No rental shop now means renting in town and lower demand, not turning round.
- 2026-10-07: [[Services]] listed with the user (unranked).
- 2026-10-07: Overnight, staff, and logistics added to [[Services]] with the user.
- 2026-10-07: [[Demo]] planned with the user and ranked first; rental shops folded into its balance step.
- 2026-10-07: Release path noted ([[Release]]).
- 2026-10-07: The user's first Boreal playtest folded into [[Demo]].
- 2026-10-08: [[Fast-Forward Performance]] profiled and planned with the user, ranked first.
- 2026-10-08: [[Fast-Forward Performance]] shipped; leftovers under Rendering and engine; skier-on-skier collisions noted under Safety. [[Demo]] is priority 1 again.
- 2026-10-08: [[Bough Snow]] planned with the user (unranked, under Rendering and engine).
- 2026-10-08: [[Building Tool]] shipped: separate Buildings and Services menus, empty floor, building and room popups.
- 2026-10-08: [[Trail Network]] planned with the user (unranked); the lift graph-rebuild gap listed under Bugs.
- 2026-10-08: [[Trail Network]] steps 1–3 built; Boreal and Kirkwood have no trails until they're redrawn (step 5).
- 2026-10-08: [[Crowd Scale]] measured and planned (unranked); tile counts cached and building checks bounded along the way.
- 2026-10-08: [[Crowd Scale]] simplification explored; falls and replanning added to Bugs.
- 2026-10-08: Diagnosed falls (beginners free-skiing the fall line, falling in a loop) and replanning (failed walking paths to an enclosed lodge door flood the map).
- 2026-10-08: Falls and door fixes: free skis from a lift top only for advanced guests keen on powder, trees or steeps (or tops with no trail); novices plan only trails at their level; a walk-to-parking step gets guests home from a lift base; each fall costs patience, and the fourth on one descent strands the guest for patrol (or a walk home on foot); doors present a step outside the building. Falls 8,733 → 667 and the day 110 s → 44 s, with failed door searches gone; found that skiers don't follow trails.
- 2026-10-08: Route costs: a skier's route prices pitch past their comfort along the line and ground off trails at their level (unless they free-roam); trail-to-trail steps aim at the junction; a step from a trail junction needs that trail at the guest's level too; the lap fallback is a searched plan, not a straight free ski. Boreal, 250 guests: falls 8,733 → ~760, skier-hours 104 → 111, no 6 s fall loops.
- 2026-10-08: Fall tracking, before changing behaviour (Sunset Blvd. may be a gameplay problem as much as a steering one): a pin over every guest who is down (red: getting up; magenta: waiting for patrol); a Falls overlay, a heat map of where guests fell today; and a "Falls today" section in the ski patrol popup (count, off-run and lift-unload counts, the three runs with the most). Falls are kept with the day's history (where, which run or lift), saved, and counted in each daily sample.
- 2026-10-08: Skier model and falls: falls slide, lie, and get up by direction and skill, with yard sales; guests are a jointed figure posed from the sim, in their own outfits (activity colours on F6).
- 2026-10-08: [[Service Quality]] planned with the user (formats, staff pools, morale, appetite, shopping); amenity quality folded into it.
- 2026-10-08: Sunset Blvd.: trail following, trail edges, comfort by skill, novices on all-green lifts; ropes and signs at splits, and trail steepness for the player (held), noted.
- 2026-10-09: [[Star Ratings]] and [[Scoreless Rating]] planned with the user: stars as levels, the satisfaction ledger removed.
- 2026-10-09: [[Season Calendar]] planned with the user (unranked).
- 2026-10-09: [[Readable Complaints]] noted with the user after the Boreal Christmas check.
- 2026-10-09: [[Groups]] planned with the user, as the way toward 10,000 visitors a day ([[Crowd Scale]]).
- 2026-10-09: Turbo and the storm skip removed (the user: they were for the long calendar); "More fast-forward targets" dropped with them.
- 2026-10-09: Car kinds shipped in [[Transit]]; [[Large Vehicles]] (buses, RVs) noted as an idea.
- 2026-10-10: [[Economy Balance]] planned with the user.
- 2026-10-10: Footpaths shipped ([[Pathfinding]]): the Path tool, twice-as-fast walking, and walking routes that prefer paths.
- 2026-10-10: Lift staff shipped ([[Lifts]]): two operators on every lift and an optional line attendant who fills chairs; sweepers moved to [[Packed Snow]].
- 2026-10-10: Lines at service doors shipped ([[Amenities]]): single file out from the door, and guests inside hidden instead of stacked on the step.

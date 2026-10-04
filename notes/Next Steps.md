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

1. [[Graphics Base]]: steps 1–6 shipped (anti-aliasing, light balance, snow breakup, trees, haze, map edge). Left: bough snow that lingers after a storm (needs a recent-snowfall value in the weather sim), gamma-correct lighting, and post-processing.
2. **Real grooming**: corduroy that looks real. In this order:
   1. Grooming drawn from where the cat drove, not per cell ([[Hiding the Grid]] step 3).
   2. Corduroy as fine ridges lit by the sun, seams where passes overlap, turn marks at the ends, and skier tracks wearing it away ([[Grooming]]). *Needs* step 1 and the light balance from [[Graphics Base]].
3. [[Terrain Realism]]: make imported mountains look and behave like the real place. In this order:
   1. Mesh subdivision: a chunked terrain mesh, finer where a detail pass says so, on top of the 5 m sim grid.
   2. Cliffs baked into the refined mesh by a detail pass after import, with a rock mask for the shader; instanced rock meshes only if needed. *Needs* step 1.
   3. Creeks baked into the refined mesh by the same pass, so pathing follows from slope; frozen flats for lakes. *Needs* step 1.
   4. Snow that doesn't look plastic: wind texture, distinct powder, crust, ice, and slush, and ground showing through thin snow. *Needs* the light balance and snow breakup from [[Graphics Base]].
   5. Climate parameters in each scenario (latitude first), a sun term in auto-snow, and a realistic starting snowpack. *For* [[Kirkwood]]'s north and south faces.

Not ranked yet: everything below.

## Bugs

- **No one finds a ticket window**: in a headless two-day run of [[Boreal]] and [[Kirkwood]], with a ticket office and green, blue, and black trails added by the test, every guest left with "couldn't find where to buy a ticket" and none rode the lift. Possibly the same cause as the failing `internal/ai/goap` planner tests (no plan for KeepSkiing from parking), or the test's ticket office placement; check in the real game. Blocks playtesting any goals or balance ([[Tickets]], [[GOAP]]).
- **"Release cat is wrong"**: from the old `NEXT.md`; reproduce and describe, or drop ([[Grooming]]).

## Scenarios and campaign ([[Scenarios]], [[Scenario Campaign]])

- [[Scenario Goals and Rules]]: objectives, win and lose, per-scenario rules, and unlocking in order. In this order:
   1. Goal and rule data saved with the scenario, and goal progress saved in player saves. Save the resort rating (today it resets to 0.5 on load) and record it in each day's history sample.
   2. A daily check at rollover that updates progress and decides won, lost, or still playing, with entries in the [[Event Feed]].
   3. An in-game goals panel (also where the description can be reread), plus "Scenario complete" (keep playing) and "Scenario failed" (Retry, Quit to menu) panels.
   4. A Goals tab in the editor's Scenario details dialog for goals and rule switches.
   5. The no-grooming rule: hide and refuse the cat shed and snowcats, and guests judge powder instead of corduroy. Other rules ship with the scenarios that need them. *For* [[Asahidake]].
   6. Unlocking in campaign order, with progress kept in a small file next to the saves.
   7. Boreal's goals: open the resort, a guest count in one day, a decent rating through a weekend; a second lift as a bonus. *Needs* the ticket-window bug fixed and [[First Week Balance]].
- **Campaign scenarios**: build the rest of the [[Scenario Campaign]] in order, each pulling in the features on its note's Needs list. [[Kirkwood]] exists and still needs [[Terrain Realism]], skill-matched goals, and hand-drawn parcels.
- **Rating per skill level**: separate ratings from beginners, intermediates, and experts, so a resort can't please one group and ignore another. *Needs* skill wants (under Guests). *For* [[Kirkwood]].
- **Locals score**: a goodwill meter for regulars that crowds, price rises, and heavy building lower. *Needs* guests remembering their visits. *For* [[Mad River Glen]].
- **Two base areas**: separate parking, lodges, and possibly ratings for each base on one map. *For* [[Palisades Tahoe]].

## Lifts ([[Lifts]])

- [[Lift Operations]]: refuse overlapping lifts, then breakdowns and a maintenance contract, wind holds that spare the gondola, and guests riding down. Wind holds *need* wind that varies by day. Breakdowns are *for* the Turnaround idea in [[Scenario Campaign]].
- **Surface lifts**: magic carpet, T-bar, and rope tow for beginner areas. *For* [[Kirkwood]], and Portillo in [[Scenario Campaign]].
- **Long gondola spans**: few, tall towers over terrain a chair can't cross ([[Vision]]). *For* [[Palisades Tahoe]], [[Zermatt]].
- **Riders on both sides of the line**: arriving riders spawn on both sides of the lift line, not one.
- **Partly filled chairs**: chairs that don't always fill, more often with beginners in line.
- **Lines around buildings**: lift lines wrap around buildings instead of through them ([[Pathfinding]]).
- **Lift attendants**: see Staff.

## Terrain, trails, and land ([[Terrain]], [[Trails]], [[Trees]], [[Parcels]])

- [[Land and Boundaries]]: a ski area boundary, land purchase as a real decision, protected land, and protected buildings. Hand-drawn parcels are *for* [[Kirkwood]]; expensive land is *for* [[Palisades Tahoe]].
- **Trail closures and slow zones**: the player closes a run or paints a slow zone; guests respect them, mostly. *For* [[Alta]].
- **Night skiing**: light chosen trails and lifts, pay to run the lights, and stay open past dark for an evening crowd. Operating hours and the day-night cycle already exist; trail lights need a lighting approach that scales past the 16 spotlights the shader handles today ([[Rendering]]). Lit evenings also cut into the time cats have to groom ([[Grooming]]). *For* [[Boreal]], which really does run at night.
- **Backcountry**: gates through the boundary to terrain with no patrol or grooming, for experts only. *Needs* the boundary from [[Land and Boundaries]].
- **Cat trails**: easy, narrow ways down for beginners that get crowded.
- **Cat skiing**: snowcats carry advanced guests to ungroomed terrain. *For* the Revelstoke idea in [[Scenario Campaign]].
- **Guests react to trunks**: glade-loving and tree-shy guests respond to trunks nearby instead of the cell's tree cover ([[Stored Trees]]). *For* [[Asahidake]].
- **Editor glade tools**: a glade highlight and thinning slider in the [[Scenario Editor]], matching the play tool.
- **Biomes**: forested, sub-alpine, and alpine zones changing build cost, grooming quality, and injury risk.

## Snow, weather, and avalanches ([[Snow]], [[Weather]], [[Avalanche]], [[Calendar]])

- **Per-scenario weather**: daily weather rolls from the scenario's climate (snowfall by month, temperatures, storm frequency) instead of one shared climate. *Needs* the climate parameters from [[Terrain Realism]] step 5. *For* [[Killington]], [[Asahidake]].
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

- **What each skill wants**: beginners want rentals and easy terrain; intermediates want terrain plus food and places to rest; advanced skiers want terrain and no crowds. Feeds [[Demand]] and [[Satisfaction]]. *For* [[Kirkwood]].
- **Snowboarders**: guests already roll Snowboard but still ski and look like skiers.
- **Children and families**: their own guest type, arriving and moving as a group.
- **Guest goals beyond lapping**: hunt powder, find the shortest line, go to après-ski, stay near the lodge. Powder hunting is *for* [[Asahidake]].
- **Regulars**: guests who remember their last visit and come back, or don't. *For* [[Mad River Glen]].
- **Crowding**: guests notice crowded runs and lodges, not only lift lines. *For* [[Mad River Glen]].
- **Mogul lovers**: guests who seek moguls, and an expert bombing a mogul run entertaining the lift above.
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
- **Lodge storeys and styles**: more storeys, a style choice, and a shuffle button.

## Staff

- **Employees as people**: they drive in, park in an employee lot, and walk to their stations.
- **Employee goals**: [[GOAP]] for employees (get to work, take breaks, go home). *Needs* employees as people.
- **Lift attendants**: two per lift, top and bottom, with a third speeding loading on bigger chairs. Each lift already pays for two a day, but none are on the map. *Needs* employees as people.
- **Staffing amenities**: staff for rental, food court, and bar. *Needs* employees as people.
- **Employee housing**: so a resort can staff up where commuting is hard. *For* [[Zermatt]].

## Roads and arrivals ([[Parking and Roads]], [[Demand]])

- **Road closures**: a closed road means no arrivals that day. *For* [[Alta]].
- **Trains**: a second way to arrive, with no parking footprint. *For* [[Zermatt]].
- **Tunnels**: roads and paths through terrain.
- **Parking choice**: guests pick lots weighted by distance to the lifts.

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

- [[Hiding the Grid]]: smooth the parcel fence, painted overlays, and groomed runs so the 5 m cells don't show. Step 3 (grooming follows the cat) is part of Real grooming in Priority.
- **Environmental variety**: tree species, deciduous trees, dead snags, saplings, shrubs, and rocks. Rocks overlap with cliffs in [[Terrain Realism]].
- **Animation approach**: choose between procedural in the shader (as now), glTF skinned meshes, baked keyframes, or blended poses.
- **5,000 guests**: persistent buffers, culling the guest batch, lower-detail skier meshes. Big maps also lean on the terrain chunks from [[Terrain Realism]] step 2.

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

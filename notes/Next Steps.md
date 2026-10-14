---
title: Next Steps
kind: plan
status: planned
---
# Next Steps

Everything planned, in one place. A step big enough to need its own design gets a plan note in `notes/next/` and a link here; small items stay as a line under their area. When something ships, delete its line, update the system card's log, and add it to today's line in the [[Changelog]].

Each item is a **bold name**, then what it is. *Needs* lists what has to exist first; *For* names the scenarios that want it, so the [[Scenario Campaign]] doubles as a rough order. To prioritize something, move it up into Priority.

## Priority

In the order to work on them:

1. [[Lift Operations]]: daily wind, wind holds and test runs are in. Next: breakdowns as [[Lift Maintenance]] (maintenance to pay for, wind holds to override at a risk; UI to work out with the user), overlapping lifts, downloading.

## Bugs

Diagnose each first, report the cause, then fix:

- **Skiers stall at run gaps and flats**: found 2026-10-12 testing [[Lift Lines]]. Skiers stall 10 minutes or more where a run ends short of the next (Sunset Blvd. into Road Island on the Goals Test, a 50 m gap) and on the flat short of Lift4's base, turning back and forth at walking pace; and at Lift4's top, where the Goals Test's Kiss A Bear is drawn bottom to top.
- **Fallen skiers walk too far uphill**: the user, 2026-10-08, watching the game. After a fall guests walk much further back uphill than is realistic. Suspects: in a yard sale the skis stay near where the fall began (up to 0.5 + 0.2 × speed metres along the line) while the body slides on, so the guest climbs back up the whole slide for them (`knockDown`, `collectSki` in `sim/falls.go`); or the walk back up toward a route point or carrot uphill after getting up. Diagnose which before changing ([[Skiing]]).
- **Point-placed services fail silently with no free cell**: found 2026-10-07 fixing tests. `PlaceBuildingType` for a lodge, bar, or ticket office (tests, testbeds, the editor's ticket office) converts to a service building only on free cells (`placePointService`); with none (say, inside a parking lot) it quietly stays an old-style building with no tiles or doors that serves nothing, and guests looking for it go home. It should fail visibly or find room. Belongs with footpaths and pathing work ([[Pathfinding]], [[Lodge Shell]]).
- **Lift-station huts stand in the lift lines**: found 2026-10-08 measuring clearance round stations. The operator's hut in `lift_station.py` sits 3.4 m back and 5 m out on the empty-chair side, which at a base station is inside the lift-line rows (rows stack downhill in 2.5 m bands, places step out in 2 m slots: the hut is row 2, slot 3), so guests are sent to places inside it: about 1,850 one-second samples a day within 1 m of base huts on Boreal (250 guests). Walkers now sidestep station parts, but not one at the place they're walking to. Decide where the hut should go (beside the load point uphill of the rows, or further out) in the model and `world.StationXZs` together ([[Lifts]]).
- **Novices get bored with one short green**: Boreal Goals Test, 2026-10-08, after the Sunset Blvd. fixes (falls 948 → 136, gave up 108 → 1 on the user's save). Novices (skill under 0.10) now only ride lifts whose terrain is all green, so on Boreal they keep to Lift4's Kiss A Bear, and with one short run they're soon bored: 83 bored departures a day against about 25. Partly gameplay (build them gentle terrain: the magic carpet, [[Demo]] step 1); check the boredom rate for a guest with one lift once there's a bunny area to compare with ([[Skiing]]).

## Scenarios and campaign ([[Scenarios]], [[Scenario Campaign]])

- [[Scenario Goals and Rules]], tabled until gameplay improves. Done: goal and rule data with progress saved, the rating saved, the daily check with win and lose, the goals panel with win and lose panels, and the editor's Goals tab. Left, in order: Boreal's goals (decided: three lifts open; a best day of guests in the thousands; a 7-day 70% rating streak; bonus: all within season one; no deadline; numbers to confirm with the headless three-lift run once guests can be satisfied), the no-grooming rule (*for* [[Asahidake]]), and unlocking in campaign order.
- **Campaign scenarios**: build the rest of the [[Scenario Campaign]] in order, each pulling in the features on its note's Needs list. [[Kirkwood]] exists and still needs [[Terrain Realism]], skill-matched goals, and hand-drawn parcels.
- **Rating per skill level**: separate ratings from beginners, intermediates, and experts, so a resort can't please one group and ignore another. *Needs* skill wants (under Guests). *For* [[Kirkwood]].
- **Locals score**: a goodwill meter for regulars that crowds, price rises, and heavy building lower. *Needs* guests remembering their visits. *For* [[Mad River Glen]].
- **Two base areas**: separate parking, lodges, and possibly ratings for each base on one map. *For* [[Palisades Tahoe]].

## Lifts ([[Lifts]])

- [[Surface Lifts]] leftovers: place a magic carpet by Boreal's base, and Kirkwood's carpets and T-bar from OpenStreetMap; then measure novice boredom (see Bugs), falls on tows, and line waits. Their prices are under Economy.
- [[Night Sessions]] leftovers: check grooming against a 23:00 close (cats start at midnight; with a 21:00 close the mountain is empty by about 23:00), a cat waiting for a run to empty, and lodge windows glowing at night. Balance is under Economy.

- [[Lift Operations]]: refuse overlapping lifts, then breakdowns and a maintenance contract, wind holds that spare the gondola, and guests riding down. Wind holds *need* wind that varies by day. Breakdowns are *for* the Turnaround idea in [[Scenario Campaign]].
- **Riders on both sides of the line**: arriving riders spawn on both sides of the lift line, not one.
- [[Lift Lines]] extensions: good enough for now (the user, 2026-10-12): a crowd with no attendant, and with one a corral of two straight lanes merging into a short run to the load. Possible next steps, none planned yet:
  - **The attendant picks the line**: from the lift's capacity and its attendance (how long the line has run at this hour, on days like today), choose one line a chair wide, two lanes into one, or more lanes merging.
  - **Too long for the room**: a line longer than the corral's room runs on straight behind it, unroped, into whatever is there (a lot, a building): cap it, bend it round, or pick a layout that fits.
  - **Singles lane**: solo guests, and groups willing to split and meet at the top ([[Groups]]), fill the seats a group leaves empty.
  - **Patrol and ski school lane**: a short centre lane to the load; patrollers ride a real chair (their ride is hidden today), and the [[Ski School]]'s classes go through it (today they queue like any group).
  - **Readouts**: lane counts and waits in the lift popup and the line tooltip.

## Terrain, trails, and land ([[Terrain]], [[Trails]], [[Trees]], [[Parcels]])

- [[Rope Lines]] leftovers: ropes in the editor, hand-built mazes at lifts and doors, signs, slow zones and gates. The Rope tool and avoidance shipped 2026-10-10.
- **Trail steepness for the player** (held, the user 2026-10-08): a trail's steepest sustained pitch, a rating check ("steeper than a green should be in 2 places") marked on the map, and naming a fall's cause (too steep, too fast, trees). Guests never see the numbers; it's for the player.
- [[Trail Network]]: trails drawn as lines of nodes snapped to lifts, buildings, and other runs, each node with its own width; glades, bowls, and backcountry outlined as polygon zones. Steps 1–3 done (runs, the run tool, grooming clear of trees); left: areas (glades, bowls, backcountry) and redrawing Boreal's and Kirkwood's trails by hand, which have none until then.
- [[Terrain Realism]]: make imported mountains look and behave like the real place. Done: mesh subdivision, lidar import, the climate block, [[Terrain Layers]] (Boreal and Kirkwood re-imported with every layer), auto snow and trees from real data, and Kirkwood's cliffs ([[Ground Materials]]). Creeks and lakes are priority 0. Left, in order: editor brushes to smooth and flatten the ground (the road, smoothing, and erosion layers are done), thermal erosion for scree, and snow that doesn't look plastic.
- [[Creeks and Lakes]] leftovers: tune creek channels and how much is snow-bridged by eye on Kirkwood and Boreal; creeks freezing over in hard cold (from the lake model's frost); guests and pathing treating open creeks and open lakes as obstacles (and deciding whether a frozen lake is walkable); a lighter colour over shallow lake water.
- [[Ground Materials]] step 5: sim behaviour from materials. Avalanches start on loaded slopes above rock bands ([[Avalanche]]), and guests avoid rock ([[Skiing]]) except experts dropping small cliffs (the "send it" easter egg). Also where objects and guests stand on rock (`VisualElevationAt` still counts snow there).
- [[Land and Boundaries]]: a ski area boundary, land purchase as a real decision, protected land, and protected buildings. Hand-drawn parcels are *for* [[Kirkwood]]; expensive land is *for* [[Palisades Tahoe]].
- [[Gridless Drawing]]: draw parking lots, trails, and buildings as shapes instead of painting cells, keeping the grid only underneath for the sim and navigation. An idea for now: first work out what the grid buys each system.
- **Trail closures and slow zones**: the player closes a run or paints a slow zone; guests respect them, mostly. *For* [[Alta]].
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
- **Weather and guests**: weather changes arrivals and guest mood ([[Demand]], [[Moments]]).
- **Drifts on lee slopes**: wind moves snow from windward faces into lee slopes and gullies. Daily wind is in ([[Weather]]), with a direction each day.
- **Glaciers**: year-round snow at the top of high resorts. *For* [[Zermatt]].
- **Avalanche control**: explosives, closures, and barriers. *For* [[Alta]].
- **Avalanche risk overlay**: show where slopes are loaded before anything releases. *For* [[Alta]].
- [[Season Calendar]] leftovers: what happens in the off-season (skipped with a summary, or fast with building allowed), and how a day's place in the month is shown. The calendar itself shipped (6-day months, December to April, holidays, speeds as real time a day).

## Safety ([[Ski Patrol]])

- **Patrol enforces slow zones**: patrollers stand at slow zones and slow fast skiers. *Needs* slow zones.
- **Clinic**: treats injuries on site instead of sending guests home.
- **Medevac**: a helicopter for serious incidents.
- [[Snowmobile Routes]] leftovers: creeks, snowcats' transits on the same search, and guests stepping aside. Routes, driving and dispatch shipped 2026-10-10.
- **Skier-on-skier collisions**: today skiers only avoid each other (a 2.5 m danger zone when picking a line, and a swerve that clears a skier by 1.5 m); when that fails they pass through each other. A contact check like `hitsTrunk` would let two skiers collide: both fall, maybe an injury and a patrol call, more likely when fast, unskilled, or crowded. Makes crowding and mixed-skill runs visibly risky ([[Skiing]]). To be ranked by the user.

## Guests ([[GOAP]], [[Moments]], [[Guest Types]])

- **Calibrate the satisfaction ledger** (tabled by the user, 2026-10-07): every event amount and condition rate in `ai.Effects` was set for the old drifting model. On Boreal, intermediate and advanced guests carry "too easy" all day and can end at 0, and a storm day on an ungroomed green drops Cruisers from 0.53 to 0.32. Tune with a resort that has blue and black runs.
- **What each skill wants**: the terrain half is in [[Snow Tastes]]. Beginners want rentals and easy terrain; intermediates want terrain plus food and places to rest; advanced skiers want terrain and no crowds. Feeds [[Demand]] and [[Moments]]. *For* [[Kirkwood]].
- **Snowboarders**: guests already roll Snowboard but still ski and look like skiers.
- [[Groups]] leftovers: splitting up and meeting again (the strong skier skis alone and meets the others for lunch or at the car), group moments ("skied with friends", "lost the group"), and followers picking their own line within a run (undecided). Groups themselves shipped: leaders plan, followers ski their line, families and boarding together.
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
- [[Rental Shop]] staff: the shop is in ([[Service Improvements]]); staffing the morning rush is still story. Racks are in [[Ski Racks]].
- [[Star Ratings]] and [[Scoreless Rating]]: each guest leaves 1–5 stars as levels (didn't ski, skied with a need unmet, every need met; 4★ and 5★ later), with no satisfaction score behind them: needs met set the star, unweighted good and bad moments explain it.
- [[Service Quality]]: formats at different price points (food court, restaurant, snack stand, shop), staff as a pool per room (headcount for timeliness, wage for quality through morale), hungrier guests with a lunch rush, and shopping.
- **Views**: better-sited buildings attract more guests ([[GOAP]]) and can charge more.
- **Lockers**: another base service ([[Lockers]]).
- **Footpath follow-ups** ([[Pathfinding]]): paths that avoid building footprints. Footpaths, their editing, and their snow clearing shipped 2026-10-10.
- [[Ski Racks]] leftovers: skis in the snow slowing routes to the door, returning rental gear, and groups waiting at the rack. Racks and the ski change shipped 2026-10-10.
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
- [[Large Vehicles]]: buses and RVs, which don't fit one stall: long-stall rows or paired stalls, bus drop-offs (shuttles from town and hotels, [[Shuttles and Parking]]), and overnight RVs. Wanted (the user, 2026-10-14).
- **Better traffic**: merging, turning lanes, signals, and more than one entrance per lot, for resorts past a couple of thousand cars a morning. Part of [[Transit]] step 5.

## Economy ([[Finance]], [[Demand]])

- **Balance lights, snowmaking and surface lifts** (the user, 2026-10-14): none of their prices have been tuned against the economy. Surface lifts: the magic carpet $90k plus $600 a metre ($200 a day and an operator), the rope tow $40k plus $120 a metre ($150 a day), the T-bar $250k plus $140 a metre ($700 a day and two operators), against chairs. Snowmaking costs $250 a metre to put in and $2 a cubic metre of water (about $43k for the whole Goals Test on one very cold night; $1–2.5k a night for a 300 m green); lights $120 a metre of run, $60 a metre of lift and $0.15 a lit metre an hour (about $1,300 an evening for one lit lift and its runs against about $23k of revenue after 16:00; $4,500 against $50k with everything lit); the night ticket at half price. Check them against a lift's day and the [[Boreal Tutorial]]'s money, ideally in a playtest ([[Snowmaking on Runs]] step 4, [[Night Sessions]] step 5, [[Economy Balance]]).
- [[Money Feedback]]: a floating "-$600" when you spend, cost previews for every tool that costs money, and income and cost flashes on the cash display.
- [[Boreal Tutorial]]: the tutorial's build order and day-by-day money; first, guests have to follow what's built, and the season's length decided.
- [[Economy Balance]]: pacing in real minutes (something small every 1–3 days, a big project every week or two), payback days as the measure, a season economy report, then tuning running costs, build costs, and starting money. Open: how long a scenario should be, and whether the calendar changes.
- [[First Week Balance]]: starting cash buys a parking lot, a short lift, and a cat shed; the guest pool scales with the resort; a week of revenue buys about one lift. *Needs* the ticket-window bug fixed.
- **Full cost rebalance**: build and operating costs across the board. *Needs* [[First Week Balance]].
- **Loans**: from the old `NEXT.md`; a credit line already exists, so scope what loans add first. *For* the Turnaround idea in [[Scenario Campaign]].
- **Demand feedback**: the player can't see what prices and building do to demand, so nothing stops them raising prices on a busy day. A day ticket above the fair price only keeps some guests home, and when demand is far beyond what the lot holds (Boreal's busy days) the lot fills anyway: $60 → $90 cost about 9% of visitors and added about $9k a day, with no hit to the rating ([[Boreal Tutorial]], 2026-10-12). The ticket price stays out of reviews (the user, 2026-10-12); instead, a system that tells the player about demand, made into a game: they pay for surveys or marketing data to learn how many wanted to come, who stayed home over price, what guests think is fair, and what would draw more. To plan with the user.

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
- [[SkiFree Mode]], the rest: timed runs with a ghost, and the yeti past the boundary. Skiing it yourself, the chase camera and riding lifts are in; these are parked (the user, 2026-10-13).

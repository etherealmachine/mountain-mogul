---
title: Next Steps
kind: plan
status: planned
---

# Next Steps

Everything planned, in one place. A step big enough to need its own design gets a plan note in `notes/next/` and a link here; small items stay as a line under their area. When something ships, delete its line and update the system card's log.

## Priority

In the order to work on them:

1. [[Scenario Goals and Rules]]: objectives, win and lose, per-scenario rules, and unlocking in order. In this order:
   1. Goal and rule data saved with the scenario, and goal progress saved in player saves. Save the resort rating (today it resets to 0.5 on load) and record it in each day's history sample.
   2. A daily check at rollover that updates progress and decides won, lost, or still playing, with entries in the [[Event Feed]].
   3. An in-game goals panel (also where the description can be reread), plus "Scenario complete" (keep playing) and "Scenario failed" (Retry, Quit to menu) panels.
   4. A Goals tab in the editor's Scenario details dialog for goals and rule switches.
   5. The no-grooming rule: hide and refuse the cat shed and snowcats, and guests judge powder instead of corduroy. Other rules ship with the scenarios that need them.
   6. Unlocking in campaign order, with progress kept in a small file next to the saves.
   7. Boreal's goals: open the resort, a guest count in one day, a decent rating through a weekend; a second lift as a bonus. Targets need playtesting with [[First Week Balance]].
2. The [[Scenario Campaign]] scenarios, starting with [[Kirkwood]], each pulling in the features it needs

The rest is not ranked yet. Move items up here as they get prioritized.

## Unsorted

Collected from the old `NEXT.md`, `MVP.md`, and `DESIGN.md` (all now deleted) and the "not built yet" lines on each card.

### Bugs

- "Release cat is wrong" (from the old `NEXT.md`; needs a better description)

### Scenarios ([[Scenarios]])

- Road closures that stop arrivals, for [[Alta]]
- A rating per guest skill level, for [[Kirkwood]]

### Lifts and lines ([[Lifts]])

- Spawn arriving riders on both sides of the lift line
- Chairs that don't always fill, more often with beginners in line
- Lift lines that wrap around buildings instead of through them
- Wear, breakdowns, and a maintenance contract
- Wind holds that spare the gondola

### Base area ([[Amenities]], [[Lodge Shell]], [[Pathfinding]])

- [[Building Interiors]]: a legend for the cutaway's colors and doors, then procedurally placed furniture
- [[Rental Shop]]: the next amenity and the first staffing puzzle
- Amenities with views and quality, which change how attractive they are to [[GOAP]] and what they can charge
- Door queues and service rates
- Lockers and ski school
- Painted footpaths between buildings, with guests walking skis-off
- Ski racks where paths meet the snow
- Lodge storeys, a style choice, and a shuffle button

### Staff

- Employees as people: they drive in, park in an employee lot, and walk to their work stations
- [[GOAP]] for employees (get to work, take breaks, go home), not only guests
- Employee housing, so the resort can staff up where commuting is hard
- Lift attendants: two required (top and bottom), a third speeds loading on multi-seat chairs. Each lift is already charged for two attendants a day, but there are no attendants on the map
- Staffing for rental, food court, and bar

### Mountain ([[Weather]], [[Avalanche]], [[Trails]])

- Weather affecting arrivals and guest mood
- A daily wind direction instead of one per scenario
- Weather tailored to each scenario from real-world climate data (monthly snowfall, temperatures, storm frequency); see [[Scenario Goals and Rules]]
- Glaciers: year-round snow at the top of high resorts like [[Zermatt]]
- Avalanche control: explosives, closures, barriers
- An avalanche risk overlay before release
- Trail closures and slow zones

### Economy ([[Demand]], [[Finance]])

- "Loans" (from the old `NEXT.md`; a credit line already exists, so this needs scoping)
- [[First Week Balance]]: starting cash buys a parking lot, a short lift, and a cat shed; the guest pool scales with the resort; a week of revenue buys about one lift
- A full rebalance of build and operating costs, after First Week Balance
- Parking choice weighted by distance to the lifts
- Guests remembering their last visit

### Time ([[Calendar]])

- Fast-forward to more conditions: first freezing night, first snowfall, base depth

### Guests ([[GOAP]], [[Satisfaction]])

- What each skill wants: beginners want rentals and easy terrain; intermediates want terrain plus plenty of food and places to rest; advanced skiers want terrain and no crowds. Feeds [[Demand]], [[Satisfaction]], and [[Kirkwood]]
- Non-skiing guests who come for the other activities, the food, and the village
- Guest goals beyond lapping: hunt powder, find the shortest line, go to après-ski, stay near the lodge
- Guests who like moguls, and an expert bombing a mogul run entertaining the lift above it
- Rating feedback that names the top complaints (long lines, wrong difficulty, falls, full parking)
- A gameplay version of the follow-guest panel, with a trip history (runs taken, vertical, time on the mountain)

### Uplift and terrain ([[Lifts]], [[Trails]], [[Terrain]])

- Surface lifts for beginners: magic carpet, T-bar, rope tow
- Cat skiing for advanced guests
- Cat trails: easy ways down for beginners that get crowded
- A ski area boundary tied to land purchase ([[Parcels]]), with injuries outside patrol coverage hitting the rating hard
- Backcountry trails beyond the boundary: gates, no patrol or grooming, expert guests only (pairs with the bootpacking easter egg)
- Better cliffs: steep faces that read as rock and that guests avoid or drop
- Rivers and frozen lakes
- Glade-loving and tree-shy guests reacting to trunks nearby rather than the cell's cover ([[Trees]])
- A glade highlight and thinning slider in the editor, matching the play tool ([[Trees]])
- Biomes (forested, sub-alpine, alpine) changing build cost, grooming quality, and injury risk

### Safety ([[Ski Patrol]])

- Patrol enforcing slow zones
- A clinic that treats injuries on site
- Medevac for serious incidents

### Real estate and attractions ([[Finance]], [[Amenities]])

- Condos: a burst of income from sales, and owners who become regular guests
- Hotels: ongoing revenue, guests staying across days
- Houses: more income than condos, more land
- Zoning the base area for parking, hotels, condos, and retail
- Shops
- Non-ski attractions: an ice rink, a sledding hill, snowmobile tours, and cross-country trails

### Engine ([[Rendering]], [[Model Pipeline]])

- [[Hiding the Grid]]: smooth the parcel fence, trail and other painted overlays, and groomed runs so the 5 m cells don't show
- Choose an animation approach: procedural in the shader as now, glTF skinned meshes, baked keyframes, or blended poses
- More environmental variety: tree species, deciduous trees, dead snags, saplings, shrubs and other plants, rocks
- Terrain look: snow sparkle and blue-shifted shadows, drifts on lee slopes
- Performance toward 5,000 guests: persistent buffers, culling the guest batch, lower-detail skier meshes

### Easter eggs

Things that happen on their own when conditions are right, not placed by the player:

- Pond skimming: a warm day, a flat runout, and a puddle; an expert in a great mood crosses it and a crowd gathers
- Gaper Day: on the last weekend of the season some guests turn up in retro outfits
- Send it: a pro finds a small cliff and either stomps it or yard-sales and needs [[Ski Patrol]]
- Bootpacking: a rested pro hikes above the top lift to an untouched face, and other pros follow
- Yeti sighting: very rare, alpine terrain with low traffic and heavy snow

### New content

- Snowboarder behavior: guests already roll Snowboard, but they still ski and look like skiers (see [[Guest Types]])
- Children and families, as their own guest type ([[Guest Types]])
- Tunnels
- Trains, as a second way to arrive without a parking footprint

## Log

- 2026-10-01: Created from `NEXT.md`, `MVP.md`, `DESIGN.md`, and the cards, then deleted those three files. Only Stored Trees is ranked.
- 2026-10-01: Added the Scenarios section.
- 2026-10-02: Merged the user's Google Keep list: added First Week Balance, a Staff section, skill wants, glaciers, climate data, backcountry, cliffs and water, and more activities.
- 2026-10-02: Shipped [[Stored Trees]] and moved its leftovers into Uplift and terrain.
- 2026-10-02: Shipped [[Scenario Metadata]]. Ranked the campaign: Goals and Rules, then the scenarios.

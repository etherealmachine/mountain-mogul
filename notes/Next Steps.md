---
title: Next Steps
kind: plan
status: planned
---

# Next Steps

Everything planned, in one place. A step big enough to need its own design gets a plan note in `notes/next/` and a link here; small items stay as a line under their area. When something ships, delete its line and update the system card's log.

## Priority

In the order to work on them:

1. [[Stored Trees]]: store individual trees so the glade tool can preview and price exactly what it removes

The rest is not ranked yet. Move items up here as they get prioritized.

## Unsorted

Collected from the old `NEXT.md`, `MVP.md`, and `DESIGN.md` (all now deleted) and the "not built yet" lines on each card.

### Bugs

- "Release cat is wrong" (from the old `NEXT.md`; needs a better description)

### Lifts and lines ([[Lifts]])

- Spawn arriving riders on both sides of the lift line
- Lift attendants: two required (top and bottom), a third speeds loading on multi-seat chairs
- Chairs that don't always fill, more often with beginners in line
- Lift lines that wrap around buildings instead of through them
- Wear, breakdowns, and a maintenance contract
- Wind holds that spare the gondola

### Base area ([[Amenities]], [[Lodge Shell]], [[Pathfinding]])

- [[Rental Shop]]: the next amenity and the first staffing puzzle
- Staffing for rental, food court, and bar
- Door queues and service rates
- Lockers and ski school
- Painted footpaths between buildings, with guests walking skis-off
- Ski racks where paths meet the snow
- Lodge storeys, a style choice, and a shuffle button

### Mountain ([[Weather]], [[Avalanche]], [[Trails]])

- Weather affecting arrivals and guest mood
- A daily wind direction instead of one per scenario
- Avalanche control: explosives, closures, barriers
- An avalanche risk overlay before release
- Trail closures and slow zones

### Economy ([[Demand]], [[Finance]])

- "Loans" (from the old `NEXT.md`; a credit line already exists, so this needs scoping)
- A full rebalance of build and operating costs
- Parking choice weighted by distance to the lifts
- Guests remembering their last visit

### Time ([[Calendar]])

- Fast-forward to more conditions: first freezing night, first snowfall, base depth

### Guests ([[GOAP]], [[Satisfaction]])

- Guest goals beyond lapping: hunt powder, find the shortest line, go to après-ski, stay near the lodge
- Guests who like moguls, and an expert bombing a mogul run entertaining the lift above it
- Rating feedback that names the top complaints (long lines, wrong difficulty, falls, full parking)
- A gameplay version of the follow-guest panel, with a trip history (runs taken, vertical, time on the mountain)

### Uplift and terrain ([[Lifts]], [[Trails]], [[Terrain]])

- Surface lifts for beginners: magic carpet, T-bar, rope tow
- Cat skiing for advanced guests
- Cat trails: easy ways down for beginners that get crowded
- A ski area boundary, with injuries outside patrol coverage hitting the rating hard
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
- Non-ski attractions: an ice rink and a sledding hill

### Engine ([[Rendering]], [[Model Pipeline]])

- Choose an animation approach: procedural in the shader as now, glTF skinned meshes, baked keyframes, or blended poses
- More environmental variety: deciduous trees, dead snags, saplings, shrubs, rocks
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

- Snowboarders
- Tunnels
- Trains, as a second way to arrive without a parking footprint

## Log

- 2026-10-01: Created from `NEXT.md`, `MVP.md`, `DESIGN.md`, and the cards, then deleted those three files. Only Stored Trees is ranked.

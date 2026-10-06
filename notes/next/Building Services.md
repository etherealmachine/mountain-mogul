---
title: Building Services
kind: plan
status: shipped
---

# Building Services

Every service a resort runs out of a building (food, drinks, seating, tickets, ski patrol, the snowcat garage) works the same way: tiles in a building, a door, a capacity from its floor area, staff and daily costs, and whatever it does for guests or the mountain. Second of three steps the user set on 2026-10-06: after [[Rotated Buildings]], before fixing how services work.

## Today

- Service buildings ([[Lodge Shell]]) hold four services by tile: lounge (rest), food (meals and seating), bar (drinks), and tickets (day tickets and passes). Each connected run of one service gets a door.
- [[Ski Patrol]] and [[Grooming]] have no building: the patrol hut and equipment shed were removed on 2026-10-06 ([[Rotated Buildings]]), so nothing patrols or grooms until steps 2 and 3. Their sims remain (the rescue cycle, cat routes, sections, and nightly passes), keyed to a home building, and a testbed can still place a shed.
- Prices live on the building (meal and drink prices), and costs are split between a building base cost and per-tile costs.

## Decisions

Made with the user on 2026-10-06:

- **One service model.** Food and beverage, tickets, and ski patrol are services in a building like any other; so is the snowcat garage.
- **Services live in any building kind** ([[Rotated Buildings]]: shed, tent, lodge), subject to which kinds allow which services (open question there).

## Steps

1. Done: **A service registry.** `world.serviceInfo` lists each service's label, wall accent, floor-plan colour, tile cost, daily cost per tile, and whether guests walk in for it; `Label`, `Accent`, `TileCost`, and `TileDailyCost` read from it, and the cutaway overlay takes its colours from it. A building serves guests (is a trail destination and an anchor for the planner) only if it has a guest service.
2. Done: **Patrol as a service.** Patrol tiles (Amenities → Ski patrol, $40,000 a tile in a lodge, $250 a day) base one patroller with a snowmobile each, waiting just outside the patrol door. A rescued guest goes to the nearest patrol room ("Patrol brought ... down to first aid") and from there home to their car. The building's popup counts patrollers and how many are out.
3. Done: **Garage as a service.** Garage tiles (Amenities → Snowcat garage, $85,000 a tile in a lodge: half a cat and its bay, $30 a day plus the cats' own running costs) house one snowcat per two tiles, rounded up, parked 7.5 m out from the garage door. Grooming sections are shared among garages as they were among sheds. The popup's "Snowcats active" stepper puts cats on standby or back to work. Garage walls are blank but for the door (a roll-up door in a shed).
4. Done: **Food, drinks, lounge, tickets** read from the registry with no change in behaviour.

## How it works

- `world.SyncFleet` keeps a building's patrollers and cats in step with its tiles whenever they change (`SetTiles`); surplus patrollers go idle ones first. Removing a building removes its fleet.
- `world.ServiceHome` is where a service's vehicles wait: the middle of the cell outside its door (a cell and a half for cats), or the building's nearest door when the service has none.
- On load, cats and patrollers are restored only for buildings that still have a garage or patrol, then every building is topped up or trimmed to its tiles.
- The old `BuildingShed` and `BuildingPatrolHut` types are retired (their numbers kept so saved types hold); testbeds' `shedAt` builds a two-tile garage in a shed.
- Decided by default (change any): staffing is a flat daily cost per tile; a tile's service covers every storey; any building kind can hold any service.

## After this

Fix how services work, diagnosing first: patrol never rescuing anyone, no grooming showing, thirst repeating, almost everyone leaving exhausted, and too many falls (the Bugs and Guests items in [[Next Steps]], from the headless three-lift [[Boreal]] run).

## Open questions

- Whether some kinds should refuse some services (a garage only in a shed, no food in a shed).
- Whether storeys should add patrollers or cats (today they add seats and upkeep only).
- Staffing as real staff with wages, instead of a flat daily cost per tile.

## Log

- 2026-10-06: Planned with the user: food and beverage, tickets, ski patrol, and the snowcat garage as uniform building services.
- 2026-10-06: Built: the service registry, patrol and garage as services with fleets that follow their tiles, and the shed and hut types retired.

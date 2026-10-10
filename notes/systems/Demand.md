---
title: Demand
kind: system
status: partial
---

# Demand

Where guests come from. A fixed catchment of guests, each with a skill and a daily budget, is polled every 30 sim-seconds. Each guest at home rolls to arrive based on the resort rating, how much terrain suits them on the runs the running lifts serve (its cells, and how many runs, each saturating; [[Boreal Tutorial]]), how crowded the [[Lifts]] are, and whether the day ticket plus parking fits their budget. Arrivals follow each guest's preferred time (`Guest.ArrivalOffset`, hours after opening): a quarter are eager, arriving 1.5 to 2.5 hours early to be in line when the lifts start; half come from an hour before opening to an hour and a half after; a quarter come 1.5 to 4 hours after. Each guest's arrivals spread about 20 minutes around their time. Nobody arrives in the last hour or while the resort is closed, and the mountain opens to arrivals three hours before the lifts. Guests can line up at a lift that's switched on but not yet running, without losing patience.

The resort rating is the average final [[Moments]] of the guests who left the day before, set at midnight. A better rating draws more guests and lets the resort charge more before they balk.

A guest without a pass is turned away if no building sells [[Tickets]]. Each poll's winners from one entry share cars of one to four and drive in (see [[Transit]]); when the car parks, its guests pay their shares of the per-car fee (see [[Parking and Roads]]), and the ticket price is set aside from their budget. That money, and their spending on [[Amenities]], lands in [[Finance]].

The catchment is fixed (each road entry's pool), but how many come grows with what's built: a beginner lift draws beginners, and each lift that serves more runs draws more of the guests those runs suit. A guest's daily chance is `VisitsPerSeason × 0.1` by day type, independent of the season's length.

Not built yet: [[Weather]] affecting arrivals, per-lot weighting, guests remembering their last visit, and what each skill level wants beyond terrain (rentals, food and rest, no crowds).

Spec: [[Demand Spec]]. Code: `internal/sim/demand.go`.

## Log

- 2026-10-01: Catchment poll, rating, price response, arrival curve, ticket gate, and parking fee are in.
- 2026-10-06: Guests only come when a running lift (open, not on hold) serves their level, or any running lift for advanced guests, as the planner allows; with the resort open but every lift stopped, nobody comes and the event feed says "Guests turned away: every lift is stopped" once a day.
- 2026-10-06: The resort rating lives on the world (`World.Rating`) and is saved; it no longer resets to 50% on load.
- 2026-10-06: The guest pool comes from the road entries' pools when a map has entries (each guest lives beyond one), else the default 10,000 ([[Transit]]).
- 2026-10-06: Arrivals come in carloads that drive in from their entry ([[Transit]] step 2); guests in arriving cars count toward occupancy, and the parking share in the price factor is the fee ÷ 2.4, the mean carload.
- 2026-10-07: The rating became the day's average departing satisfaction; guests come at 0.4 rate when only easier terrain exists ([[Satisfaction Rework]]).
- 2026-10-07: Per-guest preferred arrival times; arrivals from three hours before opening.
- 2026-10-07: With no rental shop, half of the guests who'd rent skis stay home: the poll's chance is × (1 − rental share for their skill × 0.5) for guests without a pass ([[Service Improvements]]).
- 2026-10-07: Terrain match weighs the resort's marked runs against each guest's run range (their level and what their steep taste wants): fully in range, 0.3 one easier, 0.1 two easier, none harder; half the terrain suiting them draws them fully ([[Demo]] step 2).
- 2026-10-10: Demand follows what's built (`terrainDraw`): only runs a running lift serves count (`World.ReachableTrails`: off each top, plus one level of branching); a guest's draw is suitable cells ÷ (cells + 1,000) × suitable runs ÷ (runs + 2), weighted 1 in range, 0.3 one easier, 0.1 two easier. `TerrainCapacity` counts served runs only. Daily chance decoupled from the season's length ([[Season Calendar]]).
- 2026-10-10: Occupancy (`sim.Occupancy`) is the fuller of the lifts (everyone not indoors, against lift capacity) and the runs (guests on the snow, against terrain capacity); lodge guests no longer hold arrivals back ([[Boreal Tutorial]]).

---
title: First Week Balance
kind: plan
status: planned
---

# First Week Balance

Tune the opening so the first week is a clear, small loop: build a minimal resort, earn roughly one new lift's worth of money, build it, repeat. Listed in [[Next Steps]].

## Target

- **Starting money buys the minimum resort:** a parking lot, one short lift, and a shed with its first cat. Nothing more.
- **Guests scale with the resort.** The number of potential guests depends on what's built (land owned, number of lifts, lift speed, and trails), not a fixed crowd.
- **Early guests are beginners**, plus a few intermediates. Experts show up once there's terrain for them.
- **One week of revenue ≈ one new lift.**
- **Grooming matters from day one.** Without it, guests manage one run, get tired, and go home.

## Where it stands

- Starting cash is $1M, plus a $1M credit line ([[Finance]]). The minimum resort costs about $850k: parking lot $150k, a shed with one cat $200k, and a short double chair ($400k for the stations plus $150 a metre). The starting cash already roughly matches; what's left over needs deciding.
- The potential-guest pool is a fixed 10,000 people ([[Demand]]). They arrive based on the rating, terrain for their skill, crowding, and price, but the size of the resort doesn't cap how many could come.
- Skill mix is rolled once for the whole pool, so experts come from day one if there's terrain for them.
- Ungroomed snow already tires beginners six times faster and intermediates three times faster than groomed snow ([[Grooming]], [[Energy]]). Whether that adds up to "one run and done" hasn't been measured.

## Steps

1. **Measure week one.** A testbed or [[Sim Queries]] run of the minimum resort records guests per day, runs per guest, revenue, and costs, with and without a cat working ([[Testbeds]]).
2. **Size the pool from the resort.** Effective catchment grows with owned land ([[Parcels]]), lift count and speed ([[Lifts]]), and trail count and difficulty spread ([[Trails]]).
3. **Weight skill by what's built.** A beginner hill draws beginners; adding blue and black terrain opens up intermediates and experts. Ties to what each skill wants (in [[Next Steps]]).
4. **Tune to the target.** Adjust starting cash, ticket price defaults, energy drain on ungroomed snow, and pool size until one week buys about one lift. This is the first slice of the full cost rebalance.

## Open questions

- Is "one week ≈ one lift" true at every stage, or only for the first lift? Later lifts cost more, so later weeks earn more.
- Does the leftover starting money go away, or stay as a small cushion?

## Log

- 2026-10-02: Planned, from the user's notes. Current numbers checked in code.
- 2026-10-06: A headless three-lift Boreal ran about 70 guests a day with the rating near 30%: patrol never rescues, no grooming shows, thirst repeats about 35 times a visit, everyone leaves exhausted, about 3 falls a visit. The 10,000-guest pool caps any resort near 300 guests a day. Filed as Bugs and Guests items in [[Next Steps]]; these come before tuning.

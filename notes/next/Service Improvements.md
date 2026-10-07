---
title: Service Improvements
kind: plan
status: planned
---

# Service Improvements

Give each base-area service a job of its own, make price, quality, and capacity matter to guests, and add the next service types. The services work ([[Building Services]], the satisfaction ledger in [[Satisfaction]]); what's missing is a reason to build one over another, or a better one over a cheaper one. Part of [[Amenities]]. Ranked first in [[Next Steps]].

## Why

Read on 2026-10-07, after "Fix how services work" closed. What each service does today:

| Service | Stats | Score (ledger) |
|---|---|---|
| [[Food Court]] | a meal fills hunger and thirst; its seats are a rest; it pours drinks | meal +0.05, drink +0.04, rest +0.03 |
| [[Bar]] | a drink fills thirst | +0.04, the same as a food-court drink |
| [[Lounge]] | a rest fills energy and patience | +0.03 |
| [[Tickets]] | day tickets and passes | none |
| [[Ski Patrol]] | rescues | fast +0.06, slow −0.08 |
| Garage | snowcats and snowmobiles | none |

- **A bar is a food court that only pours drinks**: since food courts pour them too, it has no reason to exist.
- **Every building is the same** but for price, and price only decides whether a guest can afford it, not how they feel about it.
- **No capacity at the door** except food-court seats: no lines at a busy lodge, and no complaint about one.
- **Few service types**: a [[Rental Shop]] plan exists; ski school and lockers are one-line ideas.

## Decisions

From the user, 2026-10-07:

- **Services fulfil needs.** Needs are what a guest plans around; services declare which needs they fulfil; only the sim touches stats and the satisfaction ledger. That separates planning ([[GOAP]]) from stats.
- **Wants are needs with low urgency.** Après-ski or warming up is a need like hunger, just one that rarely outweighs skiing.
- **Guests arrive with different needs.** Each guest is rolled with their own set: some fraction of beginners need a rental shop (a hard gate), some guests want après, and so on.
- **No bathrooms for now**: enough stats already.

## The model

**A need is a kind and an urgency, 0..1.** The planner sees urgencies, never the stats behind them. Where each comes from:

| Need | Urgency from | Fulfilled by |
|---|---|---|
| Hunger | the hunger stat | food court |
| Thirst | the thirst stat | food court, bar |
| Rest | energy and patience | lodge seats, lounge |
| Rentals | rolled at arrival: no skis | [[Rental Shop]] |
| Après-ski | rolled at arrival; grows late in the afternoon, faster after a good day | bar |
| Warming up | rolled at arrival; grows with the cold | lounge |

Rentals is a hard gate, like a lift ticket: no skis, no skiing, and a guest who can't rent leaves (feeds [[Demand]]). The rest compete with skiing by urgency; wants stay low, so they win only when nothing else presses.

**One goal shape**, `FulfillNeed(kind)`, weighted by a per-need urgency curve, replaces `RelieveHunger`, `RelieveThirst`, and `Rest` and their hard-coded thresholds. Rentals gates `KeepSkiing` instead of competing with it.

**Services declare what they fulfil**: need, price, duration, capacity, and quality. One generic `UseService(building, service)` action replaces `EatAtFoodCourt`, `RelieveThirstAtBar`, and `RestAtLodge`, so a new service type declares its needs without new GOAP code.

**Choosing where to go.** Plan cost is travel + expected wait + price weighted by the guest's budget − quality, so guests walk further for a good, fairly priced lodge.

**Satisfaction on fulfilment**, scored by the sim through the ledger when the service completes, each term with a read-only thought:

- relief, scaled by how urgent the need was (a meal when starving is worth more than a snack)
- quality: good service adds, bad service takes away
- value: price against the guest's budget and the quality; "good value" adds, "overpriced" takes away, and far overpriced turns them away at the door
- wait: a line at the door drains patience, and a long one gets a thought

An unmet pressing need keeps costing per clock hour as a condition, as hunger and thirst do now.

**What it answers.** The bar's job is après-ski, whose bonus can grow with the guest's day, so a bar makes a good day end better. The rental shop gates beginners who arrive without skis. Quality is one number per building that scales relief and feeds value; how it's raised is still open.

## Steps

1. **Needs as urgencies.** Hunger, thirst, and rest become needs; services declare them; one goal shape and one generic action. A refactor: guests behave as before.
2. **Relief, quality, and value** in the ledger, with thoughts, and door lines with a rate per service.
3. **Rolled needs**: each guest arrives with their own set. Rentals first, with the [[Rental Shop]] as a building service, then après at the bar, then warming up at the lounge.

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- **How quality is raised**: rebuilding, upgrading in place (the way lifts upgrade on the same line), or something else. Step 2 can start with quality fixed per building type.
- **Rolled shares**: what fraction of beginners need rentals, and of guests want après or warming up; these probably come from the scenario's guest pool ([[First Week Balance]]).

## Log

- 2026-10-07: Written up with the user after closing "Fix how services work"; the bar's role, how quality is raised, and what rental shops do are open.
- 2026-10-07: Model settled with the user: services fulfil needs, wants are low-urgency needs, guests arrive with rolled needs (rentals as a hard gate for some beginners), no bathrooms. Planned in three steps.

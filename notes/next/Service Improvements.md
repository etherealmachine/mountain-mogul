---
title: Service Improvements
kind: plan
status: done
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

1. Done: **Needs as urgencies.** `ai.NeedKind` (hunger, thirst, rest) with an urgency per need from `world.Guest.NeedUrgency` (1 − the stat; rest takes the lower of energy and patience). `ai.Offer` (seat, meal, drink) is what a building offers, with the needs each fulfils and how long it takes in `world/offers.go` (`OfferNeeds`, `OfferDuration`, `OffersUse`, `UsePrice`, `UseDoor`). In the planner, `FulfillNeed(k)` with a per-need spec (`needSpecs`: active above, met at, weight curve, whether it preempts, the thought when blocked) replaces `RelieveHunger`, `RelieveThirst`, and `Rest` with the same thresholds and weights; `SkiToService` and `UseService(building, offer)` replace `SkiToLodge`, `SkiToBar`, `RestAtLodge`, `EatAtFoodCourt`, and `RelieveThirstAtBar`; the snapshot carries urgencies instead of hunger and thirst, and one `AtService` anchor. The sim applies an offer's stat changes and its event when it finishes (`fulfilOffer`); the planner never sees the stats behind a need. Energy and patience stay in the snapshot for the skiing goals. Plan steps save their offer (`u`).

   Checked headless: three seeds of a Boreal day on the old and new code give the same drinks per guest (0.46–0.53 both), meals (0–3), rests (1–4), and spread of guests; the runs aren't identical because equal-cost plans now break ties differently. The same eight Go tests fail before and after (all pre-existing).
2. Done: **Relief, quality, value, and lines.** Each visit scores when it ends (`sim.fulfilOffer`):
   - relief: the offer's event (meal +0.05, drink +0.04, rest +0.03) × (0.25 + the urgency of the most urgent need it met when they got in), 0.25× to 1.25×, so a trip made as a need starts pressing (0.75) counts as before
   - quality: `Building.Quality` (0..1, saved as `q`, `DefaultQuality` 0.5 everywhere until it can be raised); "what a nice place" from 0.6 up and "this place is shabby" from 0.4 down, scaled by the distance from 0.5
   - crowding: "it's packed in there" (−0.02) when the pool was 90% full
   - value: the price over what the guest expects to pay (`ExpectedPrice`: the reference price, today's defaults of $18 a meal and $8 a drink, × 0.75–1.3 by their daily budget × 0.5–1.5 by quality); "good value" (+0.02) at 0.75 or under, "way overpriced" (up to −0.04) from 1.5; at 2.5 they won't buy (`RefuseRatio`, a planner precondition), and when every place that could meet a need is that dear the planner reports "everything here costs too much", a condition (−0.05 an hour)
   - the wait: "waited ages to get served" (−0.04) after ten clock minutes in the line

   Lines at the door: each offer takes from a pool (`UsePool`): food-court seats (meals, and rests where there's no lounge; 10 a tile a floor), lounge seats (10 a tile a floor), or the counter (drinks; 4 a tile, the bar's or else the food court's). A guest who arrives to a full pool lines up at the door (`Guest.Visit`), patience draining at half the lift-line rate, and `serveLines` lets them in first come first served. The planner won't join a line as long as the pool holds (`LineOpen`) and costs the expected wait (`ExpectedWait`). `Building.Diners` is gone; `InUse` and `Waiting` per pool replace it.

   Checked headless on the user's Boreal save (one food-court tile: 10 seats, 4 at the counter), one day each:

   | Variant | Result |
   |---|---|
   | default prices | no value thoughts; 3 "packed"; rating 0.430 (0.428 before) |
   | meal $10, drink $4 | 34 "good value" |
   | quality 0.8 | 39 "nice place"; rating 0.439 |
   | meal $40, drink $20 | 36 "everything here costs too much", 43 thirsty (32 at default); rating 0.300 |
   | everyone thirsty at noon | a line of 27 at the counter, 35 waited (34 clock minutes on average), 30 "waited ages", 49 "packed"; rating 0.345 |

   Two corrections while checking: a budget factor of 0.6–1.6× made the default drink overpriced for the poorest beginners, so it's 0.75–1.3×; and crowding first lowered the quality guests priced against, so a packed visit also scored as overpriced. Value now uses the building's own quality, and crowding scores on its own.
3. Done: **Rolled needs.** `Guest.RollVisitNeeds` at arrival (shares in `world.RentalShareByTier`, `ApresShare`, `ColdShare`):
   - **Rentals** (half of beginners, 15% of intermediates, 5% of advanced; never a pass holder, who owns skis; they bring the $40 on top of their budget): urgency 1 until they rent. `JoinQueue` won't take them, so the need's goal (weight 1.2) comes first; the [[Rental Shop]] is a new tile (`ServiceRentals`), 3 guests a tile, 15 clock minutes, priced in the building popup, revenue under Rentals. With no rental shop at the resort, they rent in town on the way instead, on their own money ("had to rent skis in town", −0.02), so the resort misses the sale; and half of those who'd rent stay home (`NoRentalsStayHome` in the demand poll: about a quarter of beginners). If a shop exists but rentals become impossible or far too dear mid-day, a guest still needing skis goes home ("nowhere to rent skis", "Couldn't rent skis"). (Changed with the user the same day: at first guests with nowhere to rent turned round in the car park, which cost Boreal 44 of 89 guests.) Checked over three seeds of a Boreal day with no rental shop: 54 arrivals on average against 77 with no one staying home, about 30% fewer, since Boreal's guests are nearly all beginners; nobody turns round, and about 15 a day rent in town.
   - **Après** (30%): urgency from 14:30, reaching 0.8 × (0.5 + their score) by 16:00, and 1 once the lifts close. Weight 0.6 × urgency, so it loses to skiing, except that a guest who's done for the day (bored or spent) after 14:30 goes to the bar instead of straight home, and at closing (urgency 1) it beats going home; guests ejected from lift lines at closing replan for it too (`homeAtClosing`). At a bar only (`OfferApres`): a bar seat (8 a tile a floor) for three-quarters of an hour, two drinks' price, fills thirst; "great way to end the day" scores +0.05 × their score ÷ 0.5 (0.25× to 2×).
   - **Warming up** (40% feel the cold, `ColdSense` 0.5–1): `Chill` builds outdoors below freezing at ColdSense × (−°C) / 20 a clock hour and falls three times that fast indoors (any visit). It presses like hunger from 0.75; a lounge only (`OfferWarmUp`, a lounge seat, a quarter hour, free) clears it. "I'm freezing" is a condition from 0.85 (−0.08 an hour), "nowhere to warm up" when no lounge can be reached.

   A new `WalkToService` lets guests walk from the base area (a lot, a building, the foot of a lift) to a service building, for rentals on arrival and après after closing; hungry guests at a base lift can now walk to the lodge instead of riding up to ski to it.

   Checked headless on the user's Boreal save at −4 °C (a food court only), one day each:

   | Variant | Result |
   |---|---|
   | as is | 44 of 89 guests came without skis and turned round ("Couldn't rent skis"); rating 0.427 |
   | + rental, bar tiles | all 30 who needed skis rented ($1,200), 6 après, 0 "nowhere to rent"; rating 0.434 |
   | + rental, bar, cold ×4, no lounge | 5 "I'm freezing", 6 "nowhere to warm up"; rating 0.409 |
   | + rental, bar, lounge, cold ×4 | 6 warmed up, 4 "I'm freezing", 7 après; rating 0.441 |

   Après stays modest on Boreal because most guests leave bored before 14:30 (a scenario matter, as before this step). Fixed while checking: the new needs weren't in the planner's goal list; a guest who couldn't rent bought a season pass on arrival; the ride planner blamed the ticket window when the block was rentals; and rentals and après were booked as food revenue.

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Open questions

- **How quality is raised**: rebuilding, upgrading in place (the way lifts upgrade on the same line), or something else. Every building is at `DefaultQuality` (0.5) until this is decided; the scoring and pricing already read it.
- **Rolled shares**: set as defaults (rentals 50/15/5% by tier, après 30%, cold 40%); they probably belong to the scenario's guest pool ([[First Week Balance]]).
- **Balance**: the user is fine with the defaults for now (2026-10-07), including `NoRentalsStayHome` 0.5 (about 30% fewer guests at an all-beginner resort without a rental shop); to tune with the rest of the ledger later.

## Log

- 2026-10-07: Written up with the user after closing "Fix how services work"; the bar's role, how quality is raised, and what rental shops do are open.
- 2026-10-07: Model settled with the user: services fulfil needs, wants are low-urgency needs, guests arrive with rolled needs (rentals as a hard gate for some beginners), no bathrooms. Planned in three steps.
- 2026-10-07: Step 1: needs as urgencies, offers that fulfil them, `FulfillNeed` and `UseService`.
- 2026-10-07: Step 2: relief, quality, value, crowding, and lines at the door.
- 2026-10-07: Step 3: rolled needs (rentals and the rental shop, après at the bar, warming up in the lounge) and walking to services. Done; how quality is raised stays open.
- 2026-10-07: With no rental shop, guests rent in town (a small knock, no sale) and half of would-be renters stay home, instead of turning round in the car park.

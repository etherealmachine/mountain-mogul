---
title: Service Quality
kind: plan
status: idea
---

# Service Quality

How a service's quality and timeliness are set, so a cheap, well-staffed food court, a short-staffed bad restaurant, and everything between are all things a player can build. Answers the open question in [[Service Improvements]] (how quality is raised). Part of [[Amenities]]; the range of service types is [[Services]].

## Decisions

From the user, 2026-10-08:

- **Formats at different price points.** A need has more than one kind of service that fills it (the food court and a restaurant for meals), each with its own price point, speed, and quality ceiling.
- **Staff as a pool per room, not people.** Each service room has a headcount and a wage; quality and timeliness are the right level of complexity for now. Employees as people (commutes, housing, employee goals; see Staff in [[Next Steps]]) stay a later layer, and the pools become the headcount those people fill.
- **Staffing sets timeliness, pay sets quality.** Short staff means lines and slow service; low pay means low morale, and low morale means a poorer visit.
- **Variety waits for overnight visitors.** Repeat visits wearing off, as repeated runs do ([[Skiing]]), matters once guests stay several days.
- **Guests use more services.** More arrive hungry; a day typically takes one or two meals and a drink, and some guests want to shop.
- **For the [[Demo]]: staffing and quality only** (steps 2 and 3). Formats, appetite, shopping, and variety come after.

## The model

**Formats.** Each service format declares what it needs and what it gives:

| Format | Fills | Staff | Speed | Quality ceiling | Fair price |
|---|---|---|---|---|---|
| Food court | meals, drinks | counter staff per tile | fast | middling | low |
| Restaurant | meals, drinks; seated | servers per seats, kitchen per tile | slow | high | high |
| Snack stand | a snack (part of a meal), a hot drink | one per tile | very fast | low | very low |
| Bar | drinks, après | per tile | as now | as now | as now |
| Shop | shopping | per tile | as needed | middling | goods |

Value is judged against the format's fair price, so a $12 lunch at the food court and a $45 dinner at the restaurant can both be good value, and the same $45 at the food court is a rip-off. Exact numbers are for the steps.

**Staffing and timeliness.** A room needs a headcount from its format and size (the table's staff column). The player sets the headcount, defaulting to what it needs. The staffed share sets how many guests it serves at once and how fast; below full, lines grow and service slows, which the wait and crowding scoring already punish ([[Service Improvements]] step 2). Above full is wasted money, perhaps a little faster.

**Pay, morale, and quality.** The player sets a wage per room, defaulting to the going rate. A room's staff have one morale, 0..1, drifting toward a target set by pay against the going rate and by workload (short staff makes the rest work harder). Quality, which today sits at 0.5 everywhere, becomes the format's ceiling scaled by morale. Low morale also costs staff: people quit, the headcount falls below what was set, and hiring back takes days. That's the loop with consequences: underpay or understaff, and service gets slower and worse until it's fixed.

**Quality in the rating.** A guest's star rating gets 1.2 × the average quality of the services they used ([[Star Ratings]], [[Scoreless Rating]]), so morale and pay show up directly in the reviews. Excellent service is worth a level, and is the only way from 4★ to 5★. Lift attendants count as a service for this, so lifts need a quality too: a staff pool per lift, here rather than later.

**Cost.** A room's daily cost becomes headcount × wage plus upkeep, replacing the staff part of the flat cost per tile (`TileDailyCost`).

**Appetite.** Guests now arrive half to fully fed and eat once at most. Instead: a share arrive hungry (no breakfast, or a long drive) and eat on arrival; hunger also rises toward midday, so lunch is a rush, which is what tests staffing; a full day takes one or two meals and a drink or two. Thirst follows the same pattern.

**Shopping.** A want ([[Service Improvements]]: wants are low-urgency needs), rolled for a share of guests at arrival, rising after lunch and at the end of the day, spending from what's left of their budget, more at a better shop.

## Steps

1. **Formats.** The restaurant as a new service (table seats, slower meals, higher fair price and quality ceiling) and the snack stand; value judged against each format's fair price.
2. **Staffing and pay.** Headcount and wage per room in the room popup, with defaults; throughput from the staffed share; daily cost from headcount × wage.
3. **Morale and quality.** A morale pool per room from pay and workload; quality from the format's ceiling and morale; quits that cut headcount, and rehiring over days. Readouts in the popup (morale, short-staffed, why).
4. **Appetite.** Hungry arrivals, the lunch rush, one or two meals and a drink. Checked headless on Boreal: meals and drinks per guest, waits at lunch, food revenue, the day's rating, with a food court staffed and paid three ways.
5. **Shopping.** A shop service and the shopping want.

Each step builds with `go build` and `go vet` and is judged headless. No Go tests.

## Later

- **Variety**: ways a range of services pays off within a single day, discussed with the user (2026-10-08) and not for the demo:
  - **Food tastes**, rolled per guest like [[Snow Tastes]]: quick or linger (a powder hound wants ten minutes, a cruiser enjoys an hour), cheap or treat (follows budget: the restaurant turns away tight budgets, the food court disappoints the well-off), comfort or hot (soup and hot chocolate against a burger and a beer). A visit that matches scores better, so one kind of place leaves part of the crowd lukewarm.
  - **The day's slots**: coffee and a pastry on arrival, a fast lunch in the rush, a snack or hot chocolate mid-afternoon, après after closing. Each format suits some; a restaurant's table service is wrong for a hurried lunch and right for a long one.
  - **Weather and conditions**: hot drinks, soup, and the lounge on a cold day; a deck, BBQ, or beer garden on a sunny spring day; grab-and-go on a powder day, when time off the snow costs most. A spread of places copes with any day ([[Weather]]).
  - **Placement**: a snack or waffle hut at a lift top saves the trip to the base (plan cost already includes travel); a summit restaurant with a view is the premium place; cost rising with elevation ([[Services]]) makes what goes up there a real choice.
  - **Spreading the rush**: several smaller places, in different formats and spots, split the lunch line that one big food court gathers.
  - **Shops that fix a problem**: hand warmers on a −15° day, goggles in flat light, a tune when it's icy, rather than retail as margin alone.
  - **Groups**: a family choosing together (kids want nuggets, parents want a beer), once guests plan as a group.
  - **Repeat visits wearing off**, by building and by format, once there are overnight visitors.
- **Employees as people**: commutes, employee housing, employee goals (Staff in [[Next Steps]]); the pools here become their headcount.
- **The same pools elsewhere**: ticket windows, parking attendants, rentals, patrol. Lift attendants move into the plan, since lift rides count toward a guest's quality.

## Open questions

- **Who sets headcount**: the player directly (proposed), or pay attracting staff so a low wage leaves a room short whatever's set. The quits loop gives some of the second either way.
- **Themes within a format**: whether two food courts should differ (a burger bar and a noodle place), or formats are enough until variety matters.

## Log

- 2026-10-08: Written up with the user: formats at price points, staff as a pool per room (staffing for timeliness, pay for quality through morale), more meals, shopping; variety deferred to overnight visitors; employees as people later.
- 2026-10-08: Variety ideas noted under Later; the demo takes staffing and quality only.
- 2026-10-09: Quality feeds the star rating directly: up to 1.2 stars on top of the needs level ([[Scoreless Rating]]).

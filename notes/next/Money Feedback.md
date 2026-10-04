---
title: Money Feedback
kind: plan
status: planned
---

# Money Feedback

Make every dollar in or out visible. Today the cash number in the top bar just changes: a glade stroke, a new lift, or a day of ticket sales gives no sign of what happened or how much it cost. Part of [[UI and HUD]] and [[Finance]]. Listed in [[Next Steps]].

## What's there today

- A cost label next to the cursor, red when you can't afford it, for sheds, patrol huts, snow guns, lift stations, lifts, roads, and the glade brush.
- No preview for lodges, ticket offices, painted service-building tiles ([[Lodge Shell]]), parking lots, or land parcels before you commit.
- No feedback when money moves. Spending is about a dozen separate `World.Cash -= cost` lines in the build tools, and income is credited the same way in the sim.
- The midnight profit and loss report ([[Finance]]) is the only place income and costs are explained.

## Steps

1. **One way to spend and earn.** `World.Spend(amount, kind, pos)` and `World.Earn(amount, kind, pos)` replace the direct `Cash` edits. Each records the change in a short per-frame list (amount, category, world position if any) for the UI to read, and feeds the revenue and cost counters the day report already uses.
2. **Spend popups.** Each player purchase shows a floating red "-$600" that rises and fades at the spot where the money went (the glade brush, the new building, the lift's bottom station). Land purchases show at the parcel.
3. **Cash indicator.** The top-bar cash number flashes red on a spend and green on income. A small running delta next to it ("+$1,240") sums income over the last couple of seconds, so a stream of $8 drinks and $60 tickets reads as one rising number instead of flicker. The midnight operating-cost charge shows as one red delta.
4. **Previews everywhere.** Every tool that costs money shows its price before you commit:
   - lodges and ticket offices, like the other buildings
   - service-building tiles: the running total of the tiles being painted, including the $50,000 foundation on a new building's first tile
   - parking lots: the running total as cells are painted
   - land parcels: the price on hover with the buy tool
   - popup purchases (a snowcat, a lift upgrade): the price on the button, greyed out when unaffordable
5. **Can't afford it.** A refused purchase says why near the cursor ("Need $12,000 more") instead of doing nothing.
6. **Income sources on the map, maybe.** Faint "+$60" over ticket windows and "+$18" over food courts as guests pay, toggled by an overlay so a busy resort doesn't drown in numbers.

## Open questions

- Whether the income delta should break down by source (tickets, food, parking) on hover.
- How to show refunds, if removing a building ever gives money back.
- Whether income popups on the map are worth it, or just noise.

## Log

- 2026-10-03: Planned from the user's notes: show money leaving on the glade tool, preview building costs, and flag income and costs on the cash display.

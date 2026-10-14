---
title: Avalanche Control
kind: plan
status: planned
---

# Avalanche Control

Avalanches that matter, and a patrol that works to keep guests out of them: slide paths with a forecast danger, automatic run closures when the danger passes the player's limit, and avalanche control (mortars from the patrol building, ski cutting) to bring it down. Someone caught is a big event, and costs the resort's new Safety score. Extends [[Avalanche]] and [[Ski Patrol]]; touches [[Trails]], [[Snow]], [[Weather]], [[Demand]] and [[Star Ratings]]. *For* [[Alta]].

## What's there today

Surveyed 2026-10-14:

- **Instability is per cell** (`Cell.InstabilityScore`: top-layer water, slope between about 29° and 54°, kind, trees), and each unstable cell releases on its own coin flip; no slide paths or start zones.
- **Releases only at midnight**: `checkAvalanches` runs in the day's weather (snowfall over 4 cm of water, or rain), at the rollover, so guests are almost never caught except by the debug cheat. A caught skier is a dealbreaker and usually seriously hurt; lift riders and walkers aren't checked.
- **No levers**: no forecast, no control, no closures. There is a per-cell "Avy Risk" overlay.
- **Patrol** is one patroller per patrol tile ($40k a tile, $750 a day each), who wait at the patrol door and answer injuries.
- **No closed runs**: the only thing that takes a run out of service is the dark ([[Night Sessions]]).
- A bug: `avalanche.go` sets the wrong cell's density when the wave spreads.

## Decisions

From the user, 2026-10-14:

- **Avalanche control is an upgrade to the patrol room.** Each patrol tile holds 4 patrollers; with the upgrade they're avalanche-trained and can do control work, for extra.
- **Cost by the patroller**: $750 a day each, as now, so a full tile is $3,000; the popup sets how many of a tile's places are staffed. Avalanche-trained patrollers cost extra, and explosives cost per shot.
- **The player sets the risk** from any avalanche-control building's popup, on a five-level danger scale: Low, Moderate, Danger, High, Extreme. "Danger" (the real scale's Considerable, renamed) is the default.
- **Runs close automatically** when the danger over them is at or above that level, and reopen when it falls. This brings back run closures, but only these: the player doesn't close runs by hand.
- **Patrollers work automatically**: mortars fired from the patrol building, and ski cutting (a patroller skis the slope to set it off).
- **Someone caught** (guest or patroller) is a big event, and hurts a **Safety score** shown beside the stars for a while: it scales demand and caps the stars while it's low. Lift failures ([[Lift Maintenance]]) and crowding feed it later.

## How it works (proposed)

- **Slide paths.** At load (and when the ground changes), steep open ground (over about 29°, under two trees a cell) is grouped into start zones, and each traced down the fall line to where it runs out (`world.SlidePath`: start cells, track and runout cells, the runs it crosses). Each has a danger, Low to Extreme, from its start zone's loading (new snow and wind slab on it, the day's wind loading its lee, warming in the afternoon sun), recomputed every clock hour.
- **Releases through the day.** Natural releases roll per path every hour from its danger (rare at Moderate, likely at Extreme), so a storm or a warm afternoon can bring one down while guests are out. A release runs the existing wave down its path.
- **Closures.** A run whose cells are in a path's track or runout closes while that path's danger is at or above the limit: rope and a Closed sign at its top, guests don't plan it (as an unlit run after dark), and those on it ski out. It reopens when the danger drops below the limit.
- **Control work.** Each morning before opening, and after storms, avalanche patrollers work the paths at or over the limit, nearest first, once the runs below are clear: from the patrol building they shell paths within mortar range (about 1.5 km; a shot costs about $400); paths out of range they ski cut, which takes longer and carries a small chance of catching the patroller. A controlled release brings the snow down (and the danger with it), and the run reopens.
- **Caught.** A guest or patroller caught (on skis, walking, or in a lift line in a runout) is knocked down and buried; patrol goes to dig them out (a beacon search, longer than an injury call). The feed says so loudly, and the camera can jump there.
- **Safety score.** A resort score, 0–100%, shown beside the stars. A burial takes off a lot (about 40 points), a near miss less; it recovers a few points a day. Demand is scaled by it, and while it's under 80% the stars shown can't be over a cap.
- **Map view.** The overlay shows slide paths coloured by danger, closed runs, and mortar range from each avalanche-control building.

## Steps

1. **Slide paths and danger.** Paths from the ground, danger from loading, hourly; natural releases through the day; lift riders and walkers in a runout caught too; the density bug. The overlay redone as paths by danger.
2. **Patrol staffing.** Four places a patrol tile, cost per patroller, a staffing stepper in the popup.
3. **Closures and the risk limit.** The avalanche-control upgrade (cost, trained patrollers' extra); the limit in its popup (Danger by default); runs closing under paths at or over it, with ropes and signs, and guests keeping off.
4. **Control work.** Morning and post-storm control: mortars from the patrol building within range, ski cutting beyond; controlled releases; runs reopening.
5. **Caught and Safety.** Burials, rescues, the event; the Safety score beside the stars, in demand and capping stars.
6. **Balance**: on the Boreal Goals Test and Kirkwood, through a stormy stretch.

## Open questions

- The upgrade's price, the trained patrollers' extra, and the mortar's range and cost per shot.
- How often natural releases should catch someone with no control work: rare enough to be a shock, common enough to matter.
- Whether buildings and lifts in a runout take damage.

## Log

- 2026-10-14: Surveyed and planned with the user, ranked first.

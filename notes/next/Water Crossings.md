---
title: Water Crossings
kind: plan
status: shipped
---

# Water Crossings

Guests keep out of open water, and footpaths and roads bridge creeks and lakes on their own. Until now only the patrol's snowmobile knew water was there ([[Snowmobile Routes]] block unfrozen lakes): skiers and walkers went straight through open creeks and unfrozen lakes. Follows [[Water Editing]], which made creeks and lakes worth trusting; touches [[Pathfinding]], [[Skiing]] and [[Parking and Roads]].

## Decisions

From the user, 2026-10-13:

- **Only open water blocks.** Open creek stretches, open lake water and thin ice are a wall to guests; a snow bridge, or lake ice thick enough to stand on (the material map's ice, not thin ice), is crossed like any snow. A Covered creek or a frozen lake can be skied over.
- **Footpaths and roads bridge automatically** wherever they cross a creek or lake. Ski trails don't: a run crossing open water is crossed where it's covered, or gone round.
- **No crossing: go round, else give up.** Water is a wall to the route searches; a guest goes round by a source, a snow bridge, ice or a bridge, and when no way exists they give up that step like any unreachable goal.

## How it works

- **Open-water cells** (`world/water_cells.go`): a 5 m cell is open water when any of its material samples is open water or thin ice (`Material.Open`), unless a bridge crosses it, or it's built on (a building's or lot's ground, a road, a footpath) or within two cells of what is: built-on ground is solid, whatever a lake outline under it says. `World.RefreshOpenWater` remakes the grid when the material map's `Version` or the bridges change, once a frame before guests read it in parallel.
- **Routes** (`sim/ski_route.go`, `sim/pathfinder.go`): open-water cells are closed to the skiers' route search and the walkers' A*, as building cells are (the goal cell excepted), and neither cuts a diagonal past one; a straight line that crosses one (`Terrain.OpenWaterOnLine`, not counting its end cells) isn't clear, so a guest plans a route instead of heading straight. The snowmobile's grid blocks them too.
- **Steering** (`hazardDensityAt`, `swerve`): open-water cells are a full hazard to the fan skiers steer by, as a tree stand is, and the short-range swerve treats the water's edge like a trunk, braking hard when there's no way past; skiers' turns bend their line downhill, so one routed to a crossing off the fall line stops at the bank and edges along it. And a backstop: no guest steps from dry ground into open water (`Terrain.KeepOutOfWater`): they slide along the bank instead.
- **Cut off** (`cutOffByWater`): a guest whose route search finds no way round open water lying across their straight way, three checks in a row (about 30 s), gives up once they're within 10 m of it: down where they are and waiting for patrol, as one who's fallen over and over does (Stranded), thinking "I couldn't get across the water" (a letdown). The feed reports them once an hour, placed where the latest was ("N guests were cut off by open water in the last hour; a footpath across makes a bridge").
- **Bridges** (`world/bridge.go`, `render/bridge_mesh.go`): made from the footpaths and roads (`RebuildBridges`, on any path or road change, a water edit, a ground rebuild and on load; not saved). A line is checked for water every 0.5 m at its centre and both edges; stretches of water closer than 3 m are one bridge, reaching 1.5 m onto each bank. The deck runs straight between its ends' ground with a 0.5 m arch on spans of 12 m or more; the path or road strip, cars, and guests' figures are drawn on it (`deckOrGround`), with the deck's edges and underside below, wooden rails on posts on a footbridge and low concrete walls on a road bridge. Bridged cells are open to the route searches. The Path and Road tools' ghosts show the bridges they'd build, see-through, with the path or road on their decks (`BridgesFor`). Prices: a footbridge $150 a square metre over the path's own $10, a road bridge $1,500 a metre (`FootpathPrice`, `RoadPrice`).

## Steps

1. Done: **Guests keep out of open water**: the open-water grid, routes, the walkers' A*, the snowmobile, steering. Checked on a testbed (an open creek across a slope with a snow bridge) and on an imported map with creeks.
2. Done: **Bridges** on footpaths and roads where their line crosses water (`IsWater`: open, snow-bridged or frozen): the span over the water and a little bank either side, a deck at the banks' height, drawn (wood with rails for a path, concrete with low walls for a road), the path or road and the guests and cars on it carried over at deck height, bridged cells open to the route searches, and the bridge's metres priced into the path or road.

## Open questions

- Ski bridges for runs: not now (the user), but a run that can't be skied for open water should say so.

## Log

- 2026-10-13: Planned with the user after a probe found nothing but the snowmobile knew about water.
- 2026-10-13: Built both steps. Testbeds "Open creek across a slope" (the skier stops at the bank, edges 15 m along it and crosses on the snow bridge) and "Open creek, no crossing" (stranded 10 m short of the water after about 20 s). A first try at a speed limit at the water's edge fired on float rounding everywhere, halving falls on the Boreal Goals Test far from any water (567 to 280); limited to steps it really stops, a day there is as before (1,170 visitors, 2.84★, 597 falls, against 1,169, 2.86★, 567). On a Boreal save with fitted creeks, a 3 m path across a creek got a 5 m footbridge ($2,392 over the path's $840), a road a 5 m bridge, and the roads already there two more. The user asked for the feed line on being cut off.
- 2026-10-13: The path and road ghosts show their bridges (the user).
- 2026-10-13: A second day on the Boreal Goals Test went to 1★ for everyone (rides 0.7 a guest, "couldn't find where to buy a ticket"): a lake fitted under the base area's flat, graded ground thawed to thin ice at its shallows on the warm second day, under a parking lot and four cells from a lodge, and the walkers' search found no way from that lot to the ticket window. Found while measuring [[Families]]; it came in with this work (the commit before was 2.93★ that day). Built-on ground, and two cells round it, now never counts as open water; the day is 2.96★. The lake under the lot is a map problem for [[Water Editing]]'s shipped-scenario pass.

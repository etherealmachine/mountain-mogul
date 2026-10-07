---
title: Skiing
kind: system
status: shipped
---

# Skiing

The per-tick controller that turns a goal from [[GOAP]] into a run down the hill. Each tick it reads the slope and the line to the target, picks a heading and how hard to scrub speed, and integrates physics. Friction and edge grip come from the [[Snow]] under the skis.

Guests turn two ways. They carve at the rate their edge grip allows at their speed (lateral grip of 5 m/s² at skill 0 up to 14 at skill 1, so about 60°/s at cruising speed and slower when fast), and they pivot faster by skidding (60°/s for a beginner up to 180°/s for an expert). On any real slope a moving guest links turns across the fall line: about 45° either side for a beginner's wide traverses and 25° for an expert at their target speed, wider the faster they are going over it (up to 75°), never back uphill. Each turn holds at least 1.5 s (beginner) to 0.8 s (expert). When too fast, the turns skid round faster as a speed check. A trunk, lift tower, or other skier on the line within the next 1.5 s (at least 5 m) is dodged: the guest pivots to the nearest heading whose arc passes it (1 m from a trunk, 1.5 m from a skier, 1.6 m from a tower), or with no way past slows to a 1 m/s creep. Skidding sheds speed and costs balance for the less skilled, so a beginner who has to swerve hard can fall. The slope a guest feels is the pitch along their line (never under 60% of the full pitch), so a nervous skier copes with a steep pitch by traversing it. Guests also start slowing for a glade up to 40 m before they reach it.

A balance value drains when a guest is going too fast or too steep for their skill, scrubs hard, skis through trees, or bounces through moguls (worse the faster and the less skilled they are, easier for those who love bumps; [[Moguls]]). Steering avoids individual trunks, and whole stands less the more a guest loves glades, and a skier who heads into a trunk too fast hits it and falls, sometimes with an injury ([[Trees]]). When balance hits zero the guest falls, losing [[Energy]] and [[Satisfaction]]; a fast fall on steep ground can injure them and call [[Ski Patrol]]. Guests steer by taste: each candidate line is scored by how well the snow along it suits them (corduroy, powder, bumps, glades, steeps, ice), so Cruisers hold to corduroy and Powder Hounds and Glade Rats head into the powder and trees ([[Snow Tastes]]). Moguls are hard work: everyone but a bump skier slows down in them and heads for the smoother side of the run unless it's icy or treed ([[Moguls]]). The snow underfoot brings thoughts and tires guests faster on what they dislike, and each run's verdict scores it.

Skiing also wears the mountain: traffic packs powder, scrapes off [[Grooming]], and builds moguls. Guests don't follow painted [[Trails]]: they ski toward their next lift by their own line, routed round forest when the straight line runs through it (a path over the cells, priced by tree cover and climbing). Trails label difficulty, route the snowcats, and summarize what a lift offers.

Spec: [[Guests Spec]], the L1–L3 sections. Code: `internal/sim/skiing.go`.

## Log

- 2026-10-01: Steering, balance and falls, injuries, snow friction, and traffic wear are in.
- 2026-10-02: Steering reads nearby trunks, and skiers can hit trees.
- 2026-10-07: Fixed a guest freezing near a lodge door, swapping skis on and off forever: the take-off and put-on checks now use the same door, with a 30 m / 40 m gap.
- 2026-10-07: Steering by taste replaces the fixed corduroy pull; tree-stand avoidance eases for glade lovers. Corrected: guests never followed painted trails.
- 2026-10-07: Moguls cost balance by size and speed, scaled by (1 − skill)² and eased for bump lovers ([[Moguls]] step 4).
- 2026-10-07: Guests slow down in moguls and steer for the smoother side unless they love bumps ([[Moguls]] step 6).
- 2026-10-07: Fixed guests hitting the same tree over and over: one who fell against a trunk got up still facing it and hit it again as soon as they reached 2 m/s, up to 130 times in a day on the user's Boreal test (1,371 hits in two days, −1.6 on every guest's score). Now a hit needs closing speed toward the trunk, the trunk just hit doesn't count until they're 3 m clear, and they get up turned past it on the side nearer their way. On the same save: 115 hits, at most 12 by one guest; ratings 26% and 45% (were 15% and 32%). Left: guests on long free-skiing lines still steer through forest.
- 2026-10-07: Quick manoeuvres and real S-turns. Turning was one 40°/s cap for everyone, and turns only came from overspeed, so lines were nearly straight (0.2 linked turns a minute, about 20° either side) and guests couldn't dodge a trunk a few metres ahead. Now guests carve at a speed- and skill-dependent rate, pivot by skidding to swerve or check speed, link turns on any real slope, dodge trunks, towers, and other skiers on an arc check, and feel the pitch along their line. Free-skiing lines are routed round forest. On the user's Boreal test (three seeds, a day each): tree hits 51, 57, 56 → 1, 3, 1; beginner falls 29, 21, 14 → 24, 14, 13; on slopes guests link a turn about every 1–2 s. A precomputed clearance map wasn't needed. Left: guests traversing across a slope more than about 45° off the fall line creep at the 2 m/s floor, because edge friction treats a traverse as a skid.

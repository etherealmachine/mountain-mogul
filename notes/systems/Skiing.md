---
title: Skiing
kind: system
status: shipped
---

# Skiing

The per-tick controller that turns a goal from [[GOAP]] into a run down the hill. Each tick it reads the slope and the line to the target, picks a heading and how hard to scrub speed, and integrates physics. S-turns come out of that controller, not from a turn state. Friction and edge grip come from the [[Snow]] under the skis.

A balance value drains when a guest is going too fast or too steep for their skill, scrubs hard, or skis through trees. Steering avoids individual trunks, and whole stands less the more a guest loves glades, and a skier who heads into a trunk too fast hits it and falls, sometimes with an injury ([[Trees]]). When balance hits zero the guest falls, losing [[Energy]] and [[Satisfaction]]; a fast fall on steep ground can injure them and call [[Ski Patrol]]. Guests steer by taste: each candidate line is scored by how well the snow along it suits them (corduroy, powder, bumps, glades, steeps, ice), so Cruisers hold to corduroy and Powder Hounds and Glade Rats head into the powder and trees ([[Snow Tastes]]). The snow underfoot brings thoughts and tires guests faster on what they dislike, and each run's verdict scores it.

Skiing also wears the mountain: traffic packs powder, scrapes off [[Grooming]], and builds moguls. Guests don't follow painted [[Trails]]: they ski toward their next lift by their own line. Trails label difficulty, route the snowcats, and summarize what a lift offers.

Spec: [[Guests Spec]], the L1–L3 sections. Code: `internal/sim/skiing.go`.

## Log

- 2026-10-01: Steering, balance and falls, injuries, snow friction, and traffic wear are in.
- 2026-10-02: Steering reads nearby trunks, and skiers can hit trees.
- 2026-10-07: Fixed a guest freezing near a lodge door, swapping skis on and off forever: the take-off and put-on checks now use the same door, with a 30 m / 40 m gap.
- 2026-10-07: Steering by taste replaces the fixed corduroy pull; tree-stand avoidance eases for glade lovers. Corrected: guests never followed painted trails.

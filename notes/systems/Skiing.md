---
title: Skiing
kind: system
status: shipped
---

# Skiing

The per-tick controller that turns a goal from [[GOAP]] into a run down the hill. Each tick it reads the slope and the line to the target, picks a heading and how hard to scrub speed, and integrates physics. S-turns come out of that controller, not from a turn state. Friction and edge grip come from the [[Snow]] under the skis.

A balance value drains when a guest is going too fast or too steep for their skill, scrubs hard, or skis through trees. Steering avoids individual trunks as well as whole stands, and a skier who heads into a trunk too fast hits it and falls, sometimes with an injury ([[Trees]]). When balance hits zero the guest falls, losing [[Energy]] and [[Satisfaction]]; a fast fall on steep ground can injure them and call [[Ski Patrol]]. Terrain sets the background mood: trees, corduroy, and off-piste snow each push [[Satisfaction]] up or down depending on the guest.

Skiing also wears the mountain: traffic packs powder, scrapes off [[Grooming]], and builds moguls. Guests follow painted [[Trails]] where they exist.

Spec: [[Guests Spec]], the L1–L3 sections. Code: `internal/sim/skiing.go`.

## Log

- 2026-10-01: Steering, balance and falls, injuries, snow friction, and traffic wear are in.
- 2026-10-02: Steering reads nearby trunks, and skiers can hit trees.

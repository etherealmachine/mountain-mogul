---
title: Testbeds
kind: tooling
status: shipped
---

# Testbeds

Small named worlds that set up one behavior to watch or measure: a skier on a 10° slope, a tree patch to steer around, lunch at a [[Food Court]], a lodge mixing [[Lounge]], food, and [[Bar]] tiles. Each has a fixed seed and can adjust guests each tick, for example starting them hungry. They open the resort and set the clock to mid-morning so behavior starts at once.

Run one in the window from the testbed menu (see [[Scenes]]), or headless with `-testbed "<name prefix>"`. Headless prints a trace and a summary, and can finish with a [[Sim Queries]] query against the final state. A multi-run sweep repeats the testbed over successive seeds and tallies outcomes, for example whether skiers split evenly around a symmetric obstacle.

Code: `internal/sim/testbeds.go`, `headless.go`, `aggregate.go`.

## Log

- 2026-10-01: Testbed catalogue, the in-game menu, headless runs, end-of-run queries, and seed sweeps are in.

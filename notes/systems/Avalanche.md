---
title: Avalanche
kind: system
status: partial
---

# Avalanche

After a big snowfall or a rain day from [[Weather]], every steep cell gets an instability score from its depth, slope, snow kind, and tree cover. Wind slab and crust are the worst. Dense trees anchor the slope. Cells past the threshold may release.

A release strips that cell's [[Snow]] and sends a front downhill that widens as it goes, carries momentum through flat runouts, and stops at trees, buildings, or lift ends. A skier caught in it falls, usually with an injury, which drops [[Satisfaction]] and waits on [[Ski Patrol]]. The release is logged in the [[Event Feed]].

Not built yet: avalanche control (explosives, closures, barriers), a risk overlay before release, and avalanche history feeding reputation.

Spec: [[Snow Spec]], the avalanche section. Code: `internal/sim/avalanche.go`.

## Log

- 2026-10-01: Release, the spreading front, debris, skier injuries, and the event entry are in. Mitigation and forecasting are not.

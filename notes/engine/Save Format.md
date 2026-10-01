---
title: Save Format
kind: engine
status: shipped
---

# Save Format

A `.save` file is the whole world snapshot as gzipped msgpack: terrain and [[Snow]] per cell, buildings, [[Lifts]], [[Trails]], roads, snowcats, the guest pool and on-mountain guests, history, events, [[Parcels]], prices, the credit line, and the scenario's start date. Struct tags keep the file stable across code renames. Derived data is rebuilt on load instead of saved: the trail graph, doors, the detail texture, lift holds.

Player saves live in `~/.mountain-mogul/saves/`. Bundled scenarios in `assets/scenarios/` use the same format; the [[Scenario Editor]] writes them, and New Game starts from one.

Loading upgrades older saves where it can: older day lengths are rescaled, and single-mesh lodges and standalone bars convert to [[Lodge Shell]] tiles.

Code: `internal/save/`.

## Log

- 2026-10-01: Gzipped msgpack, player saves, bundled scenarios, and legacy conversion are in.

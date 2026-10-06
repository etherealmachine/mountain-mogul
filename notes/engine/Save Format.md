---
title: Save Format
kind: engine
status: shipped
---

# Save Format

A `.save` file is the whole world snapshot as gzipped msgpack: terrain and [[Snow]] per cell, buildings, [[Lifts]], [[Trails]], roads, snowcats, the guest pool and on-mountain guests, history, events, [[Parcels]], prices, the credit line, and the scenario's start date. Imported maps also carry where they are: the latitude/longitude bounds (`geo`), the height above sea level of elevation 0 (`base_alt`), the IANA time zone (`time_zone`), the monthly climate (`climate`), and the 1.25 m ground detail (`detail`). Scenario files from an import also keep the ground as surveyed, before terrain layers (`terrain_base`: heights in whole centimetres as per-row varint differences, the OpenStreetMap roads, and which layers are off), so the editor can switch layers and reopen the import; player games drop it on load. Drawn maps and older imports leave these out and get the defaults: 45°N on solar time, sea level, the generic climate. Struct tags keep the file stable across code renames. The scenario's name, description, and campaign placing come first in the file, so lists of saves and scenarios read them without decoding the terrain. Derived data is rebuilt on load instead of saved: the trail graph, doors, the detail texture, lift holds.

Player saves live in `~/.mountain-mogul/saves/`. Bundled scenarios in `assets/scenarios/` use the same format; the [[Scenario Editor]] writes them, and New Game starts from one.

Loading upgrades older saves where it can: older day lengths are rescaled, single-mesh lodges and standalone bars convert to [[Lodge Shell]] tiles, and per-cell tree density converts to stored [[Trees]] with the same trunk positions.

Code: `internal/save/`.

## Log

- 2026-10-01: Gzipped msgpack, player saves, bundled scenarios, and legacy conversion are in.
- 2026-10-02: Trees are saved as a list of positions; older density saves convert on load ([[Stored Trees]]).
- 2026-10-02: Scenario info fields written first, with a header-only reader ([[Scenario Metadata]]).
- 2026-10-05: Terrain detail, geographic bounds, base altitude, time zone, and climate.
- 2026-10-06: Terrain base for the editor's Layers panel (about 3.5 MB at Boreal).

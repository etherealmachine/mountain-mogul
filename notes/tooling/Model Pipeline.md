---
title: Model Pipeline
kind: tooling
status: shipped
---

# Model Pipeline

Every mesh is parametric OpenSCAD in `models-src/`. `make models` compiles each `.scad` to 3MF and then to an OBJ in `assets/models/`, keeping `color()` as vertex color and rotating from SCAD's Z-up to the game's Y-up. One SCAD unit is one metre.

Models can publish metadata with `echo("MOGUL_META", ...)`, baked into the OBJ header: seat anchors for chairs, ground footprints for buildings, and entrance anchors for doors and driveways. [[Rendering]] loads them and registers them with the world.

The [[Lodge Shell]] tile kit is built this way and shares its dimensions with the shell code. The chairlift chairs share a kit too (`models-src/lib/chair_kit.scad`): the double, quad, and 6-pack differ only in width, cushion colour, and hanger weight.

Needs the OpenSCAD development snapshot; the stable release drops colors.

Spec: [models-src/README.md](../../models-src/README.md). Converter: `tools/scad2obj/`.

## Log

- 2026-10-01: SCAD to OBJ with colors, metadata anchors, and the lodge tile kit are in.
- 2026-10-08: Chair kit for the three chairlift chairs; `make` rebuilds them when it changes.

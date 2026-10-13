---
title: Model Pipeline
kind: tooling
status: shipped
---

# Model Pipeline

Every mesh is a Python script in `models-src/` that builds its parts with build123d (OpenCascade), true curves and fillets included. `make models` runs each script in the project virtualenv (`.venv-models`, made by `make models-venv` from `models-src/requirements.txt`), and the script writes its OBJ to `assets/models/`: per-vertex colour, one flat normal per triangle, turned from the model's Z-up to the game's Y-up. One unit is one metre. `make models -j8` rebuilds all 77 in about 20 seconds; only stale ones rebuild, and a change to `models-src/lib/` rebuilds all.

`models-src/lib/mogul.py` is the shared kit: OpenSCAD-style helpers (`cube`, `cylinder(..., fn=)`, `hull`, `rotate`, `linear_extrude`, `polyhedron`), cheap rounding (`rounded_rect`, `gable`: a rounded outline extruded costs far fewer triangles than a build123d fillet), CSS colour names, and the Model that collects coloured parts and metadata and writes the OBJ. Things the game draws by the hundred keep low counts with `fn=` facets; the build prints each model's triangles.

Models publish metadata on their Model (`m.slot`, `m.footprint`, `m.part`), written into the OBJ header: seat anchors for chairs, ground footprints for buildings, spinning parts for the helicopter. [[Rendering]] loads them and registers them with the world.

The [[Lodge Shell]] tile kit (`lib/lodge_kit.py`, one `LodgeKit(kind)` for lodge, shed and tent) shares its dimensions with the shell code. The chairlift chairs share `lib/chair_kit.py`, the cars `lib/car_kit.py`.

`tools/modelcheck/check.py` compares a build against a reference set of OBJs: bounds, metadata, colours, triangle counts.

Spec: [models-src/README.md](../../models-src/README.md).

## Log

- 2026-10-01: SCAD to OBJ with colors, metadata anchors, and the lodge tile kit are in.
- 2026-10-08: Chair kit for the three chairlift chairs; `make` rebuilds them when it changes.
- 2026-10-13: Moved from OpenSCAD to build123d (the user). All 77 models ported with the same bounds, colours and metadata, checked against the OpenSCAD builds; many got smoother: round chair tubes and cushions, rounded car bodies with 16-sided wheels, rounded building corners and ridges, a rounded gondola cabin and helicopter, true-curve snow gun and helipad. Lodge tiles got cheaper (4,636 → 2,452 triangles). `tools/scad2obj` and the OpenSCAD snapshot are gone.

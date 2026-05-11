# models-src — OpenSCAD model sources

Parametric 3D models for the game, authored as OpenSCAD code. The build
pipeline (`make models`) compiles each `.scad` here into an OBJ under
`assets/models/` via an intermediate 3MF, preserving `color()` blocks as
per-vertex colour in the OBJ.

```
models-src/foo.scad  →  build/3mf/foo.3mf  →  assets/models/foo.obj
```

The conversion happens in `tools/scad2obj/main.go`, which also bridges
the coordinate-system mismatch between OpenSCAD and the game (see below).

> **Requires the OpenSCAD development snapshot** (>= 2026.04 or so) —
> the 2021.01 stable release drops `color()` information at every mesh
> export format. `brew install --cask openscad@snapshot`.

## Conventions

### Units: 1 SCAD unit = 1 metre

The renderer treats world coordinates as metres (`cellSize = 10.0` m per
terrain cell). Author models in metres so they drop into the world at
the right scale without per-mesh tweaking.

A quick imperial → metric reference for ski infrastructure:

| Imperial | Metric |
| --- | --- |
| 1 ft | 0.305 m |
| 3 ft | 0.91 m |
| 10 ft | 3.05 m |
| 15 ft | 4.57 m |
| 20 ft | 6.10 m |

### Axes: SCAD authors in Z-up, the game runs in Y-up

OpenSCAD's preview shows +Z as up. Our renderer uses +Y as up. The
converter applies a -90° rotation around the X axis at export time:

```
(x, y, z)_scad  →  (x, z, -y)_game
```

This preserves right-handedness, so face winding and lighting stay
correct. Author models naturally — Z is up while you're editing, the
game gets Y-up at build time.

By convention within SCAD source:

* **+X** is the model's "primary" horizontal direction. For a lift
  station that's the cable axis (back of the station → front of the
  station). For a tower, +X runs along the cable.
* **+Y** is the lateral / perpendicular horizontal direction. Same
  convention as the cable cross-section in the existing procedural
  tower mesh.
* **+Z** is up.

### Origin: ground plane, centred on the footprint

Place the model so its origin is at the centre of its ground footprint
on the Z=0 plane. The renderer translates the model to the cell's
elevation; anything below Z=0 in the local frame would clip into the
terrain.

### Poly counts: keep `$fn` low

Set `$fn = 8` to `$fn = 16` globally so cylinders and spheres stay
game-suitable. The renderer can handle dense meshes but agents are
spawned by the hundreds — every triangle counts in the static batch.

### Materials: `color()` flows through

`color()` blocks are preserved through the build pipeline as per-vertex
RGB in the generated OBJ. OpenSCAD's 3MF export groups triangles by the
`color()` in scope, scad2obj maps each group's `displaycolor` to a float
RGB triple, and the renderer multiplies it into the per-instance
`ColorTint` so the SCAD-authored colours are the base and the
ColorTint becomes a tint on top.

Geometry that has no `color()` wrapper renders as white (the OpenSCAD
"Default" basematerial is remapped to (1, 1, 1) by the converter so
nothing accidentally ships in OpenSCAD preview yellow).

## Real-world reference dimensions

These are real-world spec ranges for the things we model. Pick numbers
near the middle of these unless you have a specific reason to deviate.

### Skier (for scale)

* Standing height: ~1.7 m
* Width with skis: ~0.6 m
* Length with skis: ~1.8 m

### Lift tower

* Total height: 6–25 m (varies wildly with terrain; typical 12–18 m)
* Pole cross-section: 0.6–0.9 m diameter (round) or square equivalent
* Crossbar (horizontal arm at top): 4–6 m wide, 0.3 m thick

### Bullwheel (the big horizontal disc at base / top stations)

* Diameter: 3.0–4.5 m (10–15 ft)
* Thickness: ~0.3 m (1 ft)
* Axis: vertical (wheel is horizontal disc)

### Bottom / top lift station — simple surface-lift style

Reference: `~/Downloads/lift.jpg` (Norwegian T-bar bottom station).

* Single vertical column: 0.3 m × 0.9 m cross-section (1 × 3 ft),
  roughly 3 m tall (10 ft) — clears skiers but not so tall the lift
  feels overbuilt.
* Single horizontal beam from top of column toward the back: 0.3 ×
  0.9 m cross-section, length ~3.5 m (long enough to clear the
  bullwheel radius).
* Bullwheel hangs from underside of the beam at the back end. Axis
  vertical. Cables wrap around it and exit toward the **front** of
  the station (the side opposite to the bullwheel — toward the lift
  line).

### Lodge / base building

* Footprint: 15–25 m on the long side (depends on capacity)
* Height: 5–10 m for a single-storey day lodge; up to 15 m with a
  second storey

### Chair (chairlift seat)

* Per-seat width: ~0.6 m
* 2-person chair: ~1.4 m wide; 4-person: ~2.6 m
* Seat depth: ~0.5 m
* Backrest height: ~0.9 m above seat

## Workflow

1. Edit `<name>.scad`. Preview live in OpenSCAD with F5.
2. Run `make models` from the repo root. Only stale OBJs rebuild.
3. Run the game; the OBJ loader picks up the new mesh.

To author a new model, copy an existing `.scad` as a template and rename
it to match the renderer's mesh ID (e.g. `building.obj` → `building.scad`).

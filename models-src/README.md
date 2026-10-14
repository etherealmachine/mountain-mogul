# models-src — build123d model sources

Each model is a Python script that builds its 3D parts with
[build123d](https://build123d.readthedocs.io) and writes an OBJ under
`assets/models/`, with each part's colour as per-vertex colour:

```
models-src/foo.py  →  assets/models/foo.obj
```

Every script uses the kit in `lib/mogul.py`: OpenSCAD-style helpers over
build123d (`cube`, `cylinder(..., fn=)`, `hull`, `rotate`, …), named colours,
model metadata, and the OBJ writer. Shared parts live in `lib/` as Python
modules (`lodge_kit.py`, `car_kit.py`, `chair_kit.py`).

```python
from lib.mogul import *

m = Model()
m.add(cylinder(h=1.3, r=0.18, fn=6), "SaddleBrown")
m.add(translate([0, 0, 1.3], cylinder(h=4.5, r1=1.6, r2=0, fn=6)), [0.32, 0.50, 0.28])
m.main()  # writes the OBJ named on the command line
```

## Setup

`make models-venv` creates `.venv-models` (Python 3.12 or newer) with the
packages in `requirements.txt`: build123d (OpenCascade), numpy and scipy.

## Conventions

### Units: 1 unit = 1 metre

The renderer treats world coordinates as metres. Author models in metres
so they drop into the world at the right scale without per-mesh tweaking.

A quick imperial → metric reference for ski infrastructure:

| Imperial | Metric |
| --- | --- |
| 1 ft | 0.305 m |
| 3 ft | 0.91 m |
| 10 ft | 3.05 m |
| 15 ft | 4.57 m |
| 20 ft | 6.10 m |

### Axes: author in Z-up, the game runs in Y-up

Models are authored Z-up, as build123d (and OpenSCAD before it) expects.
The writer applies a -90° rotation around the X axis:

```
(x, y, z)_model  →  (x, z, -y)_game
```

This preserves right-handedness, so face winding and lighting stay
correct.

By convention:

* **+X** is the model's "primary" horizontal direction. For a lift
  station that's the cable axis (back of the station → front of the
  station). For a tower, +X runs along the cable.
* **+Y** is the lateral / perpendicular horizontal direction.
* **+Z** is up.

### Origin: ground plane, centred on the footprint

Place the model so its origin is at the centre of its ground footprint
on the Z=0 plane. The renderer translates the model to the cell's
elevation; anything below Z=0 in the local frame would clip into the
terrain.

### Curves and triangle counts

build123d builds true curves; the writer tessellates them to the Model's
tolerance (1 cm by default; `Model(tolerance=…)`, or `tolerance=` on one
part). Things the game draws by the hundred or thousand — trees, chairs,
cars, lodge tiles — must stay cheap: give their cylinders and spheres a
facet count with `fn=` (as OpenSCAD's `$fn`), and check the count the
build prints. Everything is flat-shaded, one normal per triangle.

### Colours

`m.add(shape, color)` takes a CSS colour name (`"SaddleBrown"`), `"#rrggbb"`,
or an `[r, g, b]` list in 0..1. The renderer multiplies it by the
per-instance tint, so the model's colours are the base and the tint sits
on top. Parts added without a colour are white.

## Model metadata

A script can publish anchors and attributes on its Model. The writer
turns them to game axes and writes them at the head of the OBJ as
`#`-prefixed comment lines; the renderer parses them at load time and
registers them against the mesh ID so simulation and scene code can query
them without depending on the render package.

### Passenger slots — `slot`

For ridden meshes (chairs and cabins). An anchor where a rider's feet
land; indices 0, 1, 2 … must be unique. A cabin's slots take a pose:
`"sit"` (on a bench, the hips 0.2 m back from the feet and 0.6 m up) or
`"stand"`, with `"-back"` for a rider facing the back of the cabin. Cabin
riders have no skis on (they're in the racks outside). Without a pose, a
rider sits on a chair's seat, skis on, feet on the footrest.

```python
m.slot(index, x, y, z)           # a chair's seat
m.slot(index, x, y, z, "sit-back")  # a cabin's bench, facing back
```

OBJ output: `# slot <index> <x> <y> <z> [<pose>]` (game axes).
Loader: `render.LoadOBJSlots` → `world.RegisterMeshSlots`.

### Building footprint — `footprint`

For placed buildings (lodge, shed, parking lot, …): the ground-plane
half-extents, which the placement-effects pass pads with a clearance
margin to size the apron and clear trees. Include any visible overhang
(eaves, annexes) so the apron covers the whole silhouette.

```python
m.footprint(half_x, half_y)
```

OBJ output: `# footprint <halfX> <halfY>`.
Loader: `render.LoadOBJFootprint` → `world.RegisterMeshFootprint`.

### Animated parts — `part`

A sub-mesh loaded from its own OBJ and spun each frame (the helicopter's
rotors): its name, axis (`spin_y` or `spin_z`, game axes) and pivot.

```python
m.part("helicopter_main_rotor", "spin_y", x, y, z)
```

OBJ output: `# part <name> <axis> <x> <y> <z>`.
Loader: `render.LoadOBJParts`.

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

Painted lodges are assembled from the `lodge_*.scad` tile kit rather
than one mesh. Every piece includes `lib/lodge_kit.scad`, whose
parameters (2.5 m tile, 5 m walls, 2.1 m roof rise per tile) must match
the `Shell*` constants in `internal/world/lodge_shell.go`. Wall pieces
are one tile wide with their outside face toward SCAD −Y. Roof tiles
are `roof_tile([a, b, c, d])` with each corner 0 or 1 rise units high,
corners ordered (−X,+Y), (+X,+Y), (+X,−Y), (−X,−Y). Vertex colours are
near-white on walls and roofs so the per-lodge tint shows through.

### Chair (chairlift seat)

* Per-seat width: ~0.6 m
* 2-person chair: ~1.4 m wide; 4-person: ~2.6 m
* Seat depth: ~0.5 m
* Backrest height: ~0.9 m above seat

## Workflow

1. Edit `<name>.py`. To look at a part while working, build123d's
   `show()` in an OCP viewer (ocp_vscode) or export a STEP/STL.
2. Run `make models` (or `make models -j8`) from the repo root. Only
   stale OBJs rebuild; a change to `lib/` rebuilds them all.
3. Compare against the previous build with
   `python3 tools/modelcheck/check.py <old-objs-dir> assets/models/*.obj`
   (bounds, metadata, colours, triangle counts).
4. Run the game; the OBJ loader picks up the new mesh.

To author a new model, copy an existing script as a template and name it
after the renderer's mesh (e.g. `building.obj` → `building.py`).

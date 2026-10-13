"""Lodge / base building — single-storey day lodge. Rectangular plan with
a steep gable roof so snow sheds off rather than piling up.

Sized relative to the rest of the world: a skier is ~1.7 m tall, a lift
tower is 18 m, so the lodge eaves at 5 m and the ridge at 12 m reads as a
substantial-but-not-monumental building. The wall corners and the ridge
are rounded.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
wall_w = 18.0  # long side, +X primary direction
wall_d = 12.0  # short side, +Y perpendicular
wall_h = 5.0  # wall height before the gable starts

roof_rise = 7.0  # ridge height above the wall top — gives ~49° pitch
overhang = 0.6  # eaves overhang on every side

corner_r = 0.25  # wall corners, rounded in plan
ridge_r = 0.15  # the ridge, rounded across


m = Model()

# ── Geometry ───────────────────────────────────────────────────────────
# Walls: rectangular block centred on the origin, sitting on Z = 0.
m.add(linear_extrude(rounded_rect(wall_w, wall_d, corner_r), height=wall_h), "BurlyWood")

# Roof: triangular prism running along the long axis (X), so its ridge
# line runs along X. The base is slightly larger than the walls so the
# eaves overhang.
m.add(translate([0, 0, wall_h], gable(wall_w + 2 * overhang, wall_d + 2 * overhang, roof_rise, ridge_r)),
      "DarkSlateGray")

# ── Footprint metadata ─────────────────────────────────────────────────
# Half-extents in authoring coords (X, Y) — the eaves overhang adds 0.6 m
# per side, which we include so the apron + tree-clearance zone covers the
# visible roof outline rather than just the wall plane. The writer turns
# them into the game's (halfX, halfZ) in the OBJ header for the placement
# pass.
m.footprint(wall_w / 2 + overhang, wall_d / 2 + overhang)
m.main()

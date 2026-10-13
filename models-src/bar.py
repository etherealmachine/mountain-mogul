"""Bar — compact lodge-style building, roughly half-size of the main
lodge. Rectangular plan with a steep gable roof so snow sheds off rather
than piling up. Designed for casual gathering: a warm spot near the lifts
without the scale of the full lodge. The wall corners and the ridge are
rounded.

Conventions, units, and axis docs: see models-src/README.md.

    +X = primary horizontal direction (front façade faces +X)
    +Y = lateral / perpendicular horizontal
    +Z = up (the writer turns it to game Y-up)
  Origin sits at ground plane, centred on the footprint.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
# Roughly half-size of the lodge for a secondary/annex scale:
#   Lodge: 18 m × 12 m → Bar: 9.0 m × 6.0 m
bar_w = 9.0  # long side, +X primary direction (front faces +X)
bar_d = 6.0  # short side, +Y perpendicular
bar_h = 4.5  # wall height before the gable starts

# Steep ~48° pitch so snow sheds cleanly.
roof_rise = 6.75  # ridge height above wall top
overhang = 0.5  # eaves overhang on every side

corner_r = 0.18  # wall corners, rounded in plan
ridge_r = 0.10  # the ridge, rounded across


m = Model()

# ── Geometry ───────────────────────────────────────────────────────────
# Walls: rectangular block centred on the origin, sitting on Z = 0.
m.add(linear_extrude(rounded_rect(bar_w, bar_d, corner_r), height=bar_h), "Tan")

# Roof: triangular prism running along the long axis (X). The base is
# slightly larger than the walls so the eaves overhang all sides.
m.add(translate([0, 0, bar_h], gable(bar_w + 2 * overhang, bar_d + 2 * overhang, roof_rise, ridge_r)),
      "DarkSlateGray")

# ── Footprint metadata ─────────────────────────────────────────────────
# Half-extents in authoring coords (X, Y) — the eaves overhang adds 0.5 m
# per side so the apron + tree-clearance zone covers the full visible
# silhouette including the roof edge. The writer turns them into the
# game's (halfX, halfZ) in the OBJ header for the placement pass.
m.footprint(bar_w / 2 + overhang, bar_d / 2 + overhang)
m.main()

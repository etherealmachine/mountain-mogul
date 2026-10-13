"""Equipment shed — garage-style outbuilding for snowcats and snowmobiles.
Reads as utilitarian: wider and lower than a lodge, with a shallow gable,
prominent roll-up bay doors on the front (+X) for driving equipment in and
out, and no decorative roof overhang.

Sized for two snowcat bays (each cat is ~4 m wide × ~7 m long with blade)
plus circulation space — 16 m × 12 m on the ground with 5 m wall height
(clears a cat with the blade raised). Real snow-cat barns can be 25-30 m
wide but a smaller building reads cleaner at game scale. The wall corners
and the ridge are rounded.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
wall_w = 16.0  # long side, +X primary direction (door faces +X)
wall_d = 12.0  # short side, +Y perpendicular
wall_h = 5.0  # wall height before the gable starts

roof_rise = 2.5  # shallow ~17° pitch — utilitarian, not residential

# Door geometry — two bays on the +X wall.
door_w = 5.0  # each bay's width (Y axis on the front face)
door_h = 4.0  # tall enough to clear a cat with its blade up
door_gap = 1.0  # gap between the two doors
door_inset = 0.05  # recess depth so the door reads as a panel

# Office annex — a small bump-out on the −X back wall, suggesting where
# the foreman's desk and parts shelf live.
office_w = 4.0
office_d = 3.0
office_h = 3.0

corner_r = 0.15  # wall corners, rounded in plan
ridge_r = 0.30  # the ridge, rounded across


m = Model()

# ── Geometry ───────────────────────────────────────────────────────────
# Main hall — rectangular block centred on the origin, sitting on Z = 0.
m.add(linear_extrude(rounded_rect(wall_w, wall_d, corner_r), height=wall_h), "LightSteelBlue")

# Roof: triangular prism running along the long axis. Same as the lodge
# but with no overhang, so the building reads industrial.
m.add(translate([0, 0, wall_h], gable(wall_w, wall_d, roof_rise, ridge_r)), "DimGray")

# Two bay doors on the +X face, centred on the front and separated by
# door_gap. Each door is a thin panel inset into the wall by door_inset so
# it reads as a recessed door rather than a paint mark.
half_span = door_w / 2 + door_gap / 2
m.add([translate([wall_w / 2 - door_inset / 2, y, door_h / 2], cube([door_inset, door_w, door_h], center=True))
       for y in (-half_span, half_span)], "Goldenrod")

# Office annex on the −X back wall, and a short flat-roof cap for it so
# the gabled main roof doesn't have to extend over the bump-out.
office_x = -wall_w / 2 - office_w / 2
m.add(translate([office_x, 0, 0], linear_extrude(rounded_rect(office_w, office_d, corner_r), height=office_h)), "Tan")
m.add(translate([office_x, 0, office_h], linear_extrude(rounded_rect(office_w, office_d, corner_r), height=0.2)),
      "DimGray")

# ── Footprint metadata ─────────────────────────────────────────────────
# Half-extents in authoring coords (X, Y). The office annex sticks out the
# −X back wall by office_w, so the full X extent is (wall_w/2 + office_w)
# on the back side and wall_w/2 on the front side — we use the larger of
# the two so the apron covers the full silhouette. The writer turns them
# into the game's (halfX, halfZ) in the OBJ header for the placement pass.
m.footprint(wall_w / 2 + office_w, wall_d / 2)
m.main()

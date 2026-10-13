"""Lift station — a fixed-grip chairlift's bottom / top terminal: a
housing about as wide as the chairs, from behind the bullwheel forward
over the load point, on four round posts just outside the chairs' sweep.
The bullwheel and cable are inside it and the chairs hang out of slots in
its underside, which is a little over the skiers' heads
(lib/station_kit.add_housing). An operator's hut stands beside the line.

The bullwheel is centred on the origin at cable height (BullwheelHeight in
internal/world/lift.go) with its rim at the cable gap (CableGap), so the
up and down cables, 1.5 m either side, wrap it. Chairs come round the
wheel and riders get on load_x in front of it on the riders' cable
(world.StationGeom.LoadAhead), under the roof, where a red line marks the
spot; the lift line runs back from there along the riders' cable, between
the posts.

Sides: at the base, chairs carry riders up on -Y (the game's +Z after
export) and come back empty on +Y, so the hut stands on +Y. At the top the
station is turned round and riders arrive on +Y. Keep the footprint in
step with world.fixedStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from build123d import Box, fillet

from lib.mogul import *
from lib.station_kit import *

# ── Dimensions (metres) ────────────────────────────────────────────────
seat_w = 2.40  # the widest fixed-grip chair (the quad)
post_y = sweep_half(seat_w) + 0.45  # posts just outside the chairs' sweep
housing_half = post_y + 0.2  # the housing over them
housing_back = -3.0  # behind the wheel's rim
housing_front = 5.0  # in front of the load point
post_xs = (-2.2, 4.4)  # behind the wheel and in front of the load point
load_x = 3.0  # world.StationGeom.LoadAhead

# Operator hut beside the empty side of the line.
hut_x = -3.4
hut_y = 5.0

m = Model()
add_housing(m, housing_back, housing_front, housing_half)
# Drive housing on top, behind the wheel.
drive = fillet(Box(1.6, 1.4, 0.6).edges().filter_by(lambda e: e.center().Z > 0.15), 0.12)
m.add(translate([-1.8, 0, housing_wall + housing_rise * 0.6 + 0.25], drive), "DimGray", tolerance=0.03)
for x in post_xs:
    for y in (-post_y, post_y):
        add_post(m, x, y, housing_bottom + 0.3)
add_load_lines(m, load_x)
add_hut(m, hut_x, hut_y, door_back=True)
m.main()

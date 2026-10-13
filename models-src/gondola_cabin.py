"""Monocable detachable (MDG) gondola cabin — 8 passengers in two rows of
four.

Conventions, units, and axis docs: see models-src/README.md.

    +X = forward (direction of travel along the cable)
    +Y = lateral (perpendicular to cable)
    +Z = up (the writer turns it to the game's Y-up)

Origin sits at the cable-grip point. The entire cabin hangs in -Z. The
hanger arm runs from the grip down to the cabin roof; the body hangs below
that. The body is one rounded shell: corners rounded top to bottom, the
roof edge rounded over, a window band round the long sides. Keep colours
near white so the per-instance tint (grey = empty, blue = occupied) reads
cleanly.
"""

import numpy as np

from lib.mogul import *


def polyhedron_hull(points):
    """The convex hull of a point list, as a solid."""
    return hull_points(np.array(points))


# ── Dimensions (metres) ────────────────────────────────────────────────
hanger_thick = 0.14  # cross-section of the grip arm
hanger_len = 0.80  # grip to cabin roof

cabin_w = 2.40  # lateral (Y) — fits 4 riders side-by-side at 0.60 m spacing
cabin_d = 1.30  # along-cable (X) — two facing rows with ~0.30 m knee clearance
cabin_h = 2.10  # floor to ceiling
wall_t = 0.08  # wall thickness
corner_r = 0.28  # vertical corners, rounded
roof_r = 0.14  # roof edge, rounded over

# Divide the side-wall height into three bands.
lower_frac = 0.28  # solid skirt below windows
win_frac = 0.47  # window strip
upper_frac = 0.25  # solid header above windows (= 1 - lower - win)

lower_h = cabin_h * lower_frac  # 0.588
win_h = cabin_h * win_frac  # 0.987
upper_h = cabin_h * upper_frac  # 0.525

# Z anchors (everything hangs in -Z).
cabin_top_z = -hanger_len  # -0.80
cabin_bot_z = cabin_top_z - cabin_h  # -2.90
win_bot_z = cabin_bot_z + lower_h  # -2.312
win_top_z = win_bot_z + win_h  # -1.325
cabin_ctr_z = (cabin_top_z + cabin_bot_z) / 2  # -1.85

m = Model()

# ── Hanger ─────────────────────────────────────────────────────────────
m.add(translate([0, 0, -hanger_len], cylinder(h=hanger_len, r=hanger_thick / 2, fn=10)), "white")

# ── Cabin ──────────────────────────────────────────────────────────────
# Kept faceted (the game draws one per cabin on every gondola): the plan is
# a rectangle with its corners rounded in a few steps, and the roof edge
# rolls over in two steps.


def rounded_rect(hx, hy, r, steps=3):
    pts = []
    for cx, cy, a0 in ((hx - r, hy - r, 0), (-hx + r, hy - r, 90), (-hx + r, -hy + r, 180), (hx - r, -hy + r, 270)):
        for i in range(steps + 1):
            a = math.radians(a0 + 90 * i / steps)
            pts.append((cx + r * math.cos(a), cy + r * math.sin(a)))
    return pts


def plan_at(inset, z):
    return [(x, y, z) for x, y in rounded_rect(cabin_d / 2 - inset, cabin_w / 2 - inset, corner_r - inset)]


walls_top = cabin_top_z - roof_r
body = union(
    translate([0, 0, cabin_bot_z], linear_extrude(rounded_rect(cabin_d / 2, cabin_w / 2, corner_r), walls_top - cabin_bot_z)),
    polyhedron_hull(plan_at(0, walls_top - 0.001) + plan_at(roof_r * 0.3, cabin_top_z - roof_r * 0.3)
                    + plan_at(roof_r, cabin_top_z)),
)

# The window strip along each long side, running round into the corners
# between the front and rear walls; the rest of the shell is white.
inner_w = cabin_d - 2 * wall_t
band = translate([0, 0, (win_bot_z + win_top_z) / 2], cube([inner_w, cabin_w + 0.1, win_h], center=True))
m.add(body - band, "white", tolerance=0.02)
m.add(body & band, [0.70, 0.88, 1.00], tolerance=0.02)

# ── Passenger seat anchors ─────────────────────────────────────────────
# Eight riders in two rows of four (front row at +X, rear at -X).
# Riders stand; slot_z is the cabin floor level (just above the floor panel).
row_x_front = 0.35
row_x_rear = -0.35
slot_y_outer = 0.90
slot_y_inner = 0.30
slot_z = cabin_bot_z + wall_t  # -2.82 — standing on the floor

# Front row (slots 0–3, left to right in travel direction).
m.slot(0, row_x_front, -slot_y_outer, slot_z)
m.slot(1, row_x_front, -slot_y_inner, slot_z)
m.slot(2, row_x_front, slot_y_inner, slot_z)
m.slot(3, row_x_front, slot_y_outer, slot_z)
# Rear row (slots 4–7).
m.slot(4, row_x_rear, -slot_y_outer, slot_z)
m.slot(5, row_x_rear, -slot_y_inner, slot_z)
m.slot(6, row_x_rear, slot_y_inner, slot_z)
m.slot(7, row_x_rear, slot_y_outer, slot_z)
m.main()

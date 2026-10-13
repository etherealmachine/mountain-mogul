"""Helicopter body — static parts only. Main and tail rotors are separate
animated parts (helicopter_main_rotor.py, helicopter_tail_rotor.py)
declared with m.part below.

The cabin is one rounded shell: rounded-rectangle sections hulled from the
rounded tail end through the cabin to a blunt nose, with the windscreen
laid over the nose's upper face. The tail boom tapers from the cabin's
back to a rounded end under a swept fin.

Conventions, units, and axis docs: see models-src/README.md.
"""

import numpy as np

from lib.mogul import *

# ── Dimensions (keep in sync with helicopter_main_rotor.py and
#    helicopter_tail_rotor.py) ──────────────────────────────────────────

body_cx = 0.8
body_l = 4.5
body_w = 1.8
body_h = 2.0
body_z = 0.65

nose_l = 1.0
nose_w = 1.4
nose_h = 1.3

boom_l = 5.0
boom_r1 = 0.38
boom_r2 = 0.20
boom_rise = 0.50

fin_h = 1.10
fin_l = 0.85
fin_thk = 0.12

stab_span = 1.8
stab_l = 0.65
stab_thk = 0.10

skid_l = 4.0
skid_r = 0.065
skid_y = 0.82
strut_r = 0.05

mast_h = 0.50
mast_r = 0.10

# Derived
body_front = body_cx + body_l / 2
body_rear = body_cx - body_l / 2
body_top = body_z + body_h
boom_end_x = body_rear - boom_l
boom_z_fwd = body_z + body_h * 0.38
boom_z_end = boom_z_fwd + boom_rise
mast_top = body_top + mast_h

# Tail rotor hub position (must match helicopter_tail_rotor.py)
tr_x = boom_end_x + fin_l * 0.50
tr_y = fin_thk / 2 + 0.06
tr_z = boom_z_end + fin_h * 0.65

body_color = [0.85, 0.12, 0.10]
tail_color = [0.72, 0.10, 0.08]
glass_color = [0.06, 0.10, 0.20]

m = Model()

# ── Animated part declarations ─────────────────────────────────────────
# Written as "# part <name> <axis> <x> <y> <z>" in the OBJ header
# (game-space coordinates). The renderer loads <name>.obj and positions
# one instance per helicopter at body_pos + rotY(heading)*offset.
m.part("helicopter_main_rotor", "spin_y", body_cx, 0, mast_top)
m.part("helicopter_tail_rotor", "spin_z", tr_x, tr_y, tr_z)


# ── Shapes ─────────────────────────────────────────────────────────────
def hull_of(points):
    """The convex hull of a point list, as a solid."""
    return hull_points(np.array(points, dtype=float))


def section(x, w, h, zc, r, steps=4):
    """A rounded-rectangle section across the fuselage at x: w wide, h
    tall, centred on height zc, corners of radius r."""
    pts = []
    for cy, cz, a0 in ((w / 2 - r, h / 2 - r, 0), (-w / 2 + r, h / 2 - r, 90),
                       (-w / 2 + r, -h / 2 + r, 180), (w / 2 - r, -h / 2 + r, 270)):
        for i in range(steps + 1):
            a = math.radians(a0 + 90 * i / steps)
            pts.append((x, cy + r * math.cos(a), zc + cz + r * math.sin(a)))
    return pts


def fuselage_pts(grow=0.0):
    """The cabin and nose as sections, grown outward by grow."""
    g = grow
    zc = body_z + body_h / 2
    nz = zc - 0.25
    tip = body_front + nose_l
    return (section(body_rear - g, body_w - 0.4 + 2 * g, body_h - 0.4 + 2 * g, zc, 0.25 + g)
            + section(body_rear + 0.2, body_w + 2 * g, body_h + 2 * g, zc, 0.45 + g)
            + section(body_front, body_w + 2 * g, body_h + 2 * g, zc, 0.45 + g)
            + section(tip - 0.25, nose_w + 0.1 + 2 * g, nose_h + 0.1 + 2 * g, nz + 0.02, 0.4 + g)
            + section(tip + g, nose_w - 0.3 + 2 * g, nose_h - 0.3 + 2 * g, nz, 0.25 + g))


# ── Fuselage, nose and windscreen ──────────────────────────────────────
m.add(hull_of(fuselage_pts()), body_color)
# The windscreen: a skin just proud of the nose's upper face, from the
# cabin's front round onto the nose.
screen = hull_of(fuselage_pts(0.015)) & translate(
    [body_front - 0.25, -nose_w * 0.43, body_z + body_h * 0.33], cube([nose_l * 1.2, nose_w * 0.86, body_h]))
m.add(screen, glass_color)
# Side windows in the cabin doors.
for s in (-1, 1):
    m.add(translate([body_cx + 1.1, s * (body_w / 2 - 0.01), body_z + body_h * 0.62],
                    cube([1.3, 0.04, 0.7], center=True)), glass_color)

# ── Tail boom, fin and stabiliser ──────────────────────────────────────
m.add(hull(translate([body_rear, 0, boom_z_fwd], sphere(r=boom_r1, fn=14)),
           translate([boom_end_x, 0, boom_z_end], sphere(r=boom_r2, fn=14))), [0.78, 0.10, 0.08])
# Fin: swept back at the top, upright at the tail end.
fin = [(0, 0), (fin_l, 0), (fin_l * 0.68, fin_h), (0, fin_h)]
m.add(translate([boom_end_x, fin_thk / 2, boom_z_end], rotate([90, 0, 0], linear_extrude(fin, fin_thk))),
      tail_color)
# Horizontal stabiliser, its tips cut back.
stab_cx = boom_end_x + stab_l * 0.55
half = stab_span / 2
stab = [(-stab_l / 2, -half), (stab_l / 2 - 0.2, -half), (stab_l / 2, -half + 0.3),
        (stab_l / 2, half - 0.3), (stab_l / 2 - 0.2, half), (-stab_l / 2, half)]
m.add(translate([stab_cx, 0, boom_z_end + 0.05], linear_extrude(stab, stab_thk, center=True)), tail_color)

# ── Rotor mast ─────────────────────────────────────────────────────────
m.add(translate([body_cx, 0, body_top], cylinder(h=mast_h, r=mast_r, fn=12)), [0.28, 0.28, 0.30])

# ── Skids and struts ───────────────────────────────────────────────────
skids = []
for s in (-1, 1):
    y = s * skid_y
    skids.append(rod([body_cx - skid_l / 2, y, skid_r], [body_cx + skid_l / 2, y, skid_r], skid_r, fn=10))
    # Turned up at the front.
    skids.append(translate([body_cx + skid_l / 2, y, skid_r], sphere(r=skid_r, fn=10)))
    skids.append(rod([body_cx + skid_l / 2, y, skid_r], [body_cx + skid_l / 2 + 0.28, y, skid_r + 0.22], skid_r,
                     fn=10))
m.add(skids, [0.18, 0.18, 0.20])
struts = []
for sx in (0.9, -0.9):
    for s in (-1, 1):
        struts.append(rod([body_cx + sx, s * body_w * 0.37, body_z + 0.1], [body_cx + sx, s * skid_y, skid_r],
                          strut_r, fn=8))
m.add(struts, [0.22, 0.22, 0.24])
m.main()

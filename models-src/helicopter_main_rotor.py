"""Helicopter main rotor — 3-blade assembly. Rendered as a separate
animated part that spins around the game-Y (vertical) axis. Origin is at
the rotor hub centre so the renderer can spin all vertices around (0,0,0).

Keep dimensions in sync with helicopter_body.py.
"""

from lib.mogul import *

body_cx = 0.8
body_z = 0.65
body_h = 2.0
body_top = body_z + body_h
mast_h = 0.50
mast_top = body_top + mast_h

blade_l = 5.0
blade_w = 0.30
blade_h = 0.07
tip_r = 0.12  # blade tips rounded in plan
n_main = 3

def blade_plan(length, width, r, steps=4):
    """A blade's outline from the hub out along +X, its tip corners
    rounded in a few steps."""
    hw = width / 2
    pts = [(0, -hw)]
    for cy, a0 in ((-hw + r, -90), (hw - r, 0)):
        for i in range(steps + 1):
            a = math.radians(a0 + 90 * i / steps)
            pts.append((length - r + r * math.cos(a), cy + r * math.sin(a)))
    pts.append((0, hw))
    return pts


m = Model()
# Everything is built about the hub centre (body_cx, 0, mast_top), the
# origin here.

# Hub disc.
m.add(cylinder(h=0.14, r=0.24, center=True, fn=16), [0.32, 0.32, 0.35])

# Blades — each extends from hub centre outward in +X, 120° apart, tips
# rounded.
blade = linear_extrude(blade_plan(blade_l, blade_w, tip_r), blade_h, center=True)
blades = [rotate([0, 0, i * 360 / n_main], translate([0, 0, 0.08], blade)) for i in range(n_main)]
m.add(blades, [0.12, 0.12, 0.14])
m.main()

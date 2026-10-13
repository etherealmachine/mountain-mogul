"""Helicopter tail rotor — 2-blade assembly. Rendered as a separate
animated part that spins around the game-Z axis (= authoring Y = the
tail-boom shaft axis). Origin is at the rotor hub centre so the renderer
can spin all vertices around (0,0,0).

Keep dimensions in sync with helicopter_body.py.
"""

from lib.mogul import *

body_z = 0.65
body_h = 2.0
body_cx = 0.8
body_l = 4.5
body_rear = body_cx - body_l / 2
boom_l = 5.0
boom_end_x = body_rear - boom_l
boom_rise = 0.50
body_top = body_z + body_h
boom_z_fwd = body_z + body_h * 0.38
boom_z_end = boom_z_fwd + boom_rise

fin_h = 1.10
fin_l = 0.85
fin_thk = 0.12

tr_l = 0.80
tr_w = 0.18
tr_h = 0.05
tip_r = 0.07  # blade tips rounded
n_tr = 2

# Hub position in authoring space (matches helicopter_body.py); the model
# is built about it, so it is the origin here.
tr_x = boom_end_x + fin_l * 0.50
tr_y = fin_thk / 2 + 0.06
tr_z = boom_z_end + fin_h * 0.65

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

# Hub disc — axis along Y (shaft direction).
m.add(rotate([90, 0, 0], cylinder(h=0.10, r=0.12, center=True, fn=16)), [0.32, 0.32, 0.35])

# Two blades, 180° apart, extending along X from hub centre, tips rounded.
# The plan is drawn in X-Y and stood up so its width runs along Z.
blade = rotate([90, 0, 0], linear_extrude(blade_plan(tr_l, tr_w, tip_r), tr_h, center=True))
blades = [rotate([0, i * 360 / n_tr, 0], blade) for i in range(n_tr)]
m.add(blades, [0.12, 0.12, 0.14])
m.main()

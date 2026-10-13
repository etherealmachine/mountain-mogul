"""Roof rack with skis — two crossbars and three pairs of skis clamped
tips forward, set on a car's roof (render.carRoofs gives each kind's roof
height). Origin at the rack's centre on the roof. Drawn with a white
tint, so the skis keep their colours.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

bar_w = 1.50
rack = [0.12, 0.12, 0.12]

m = Model()
for x in (-0.45, 0.45):
    m.add(translate([x - 0.03, -bar_w / 2, 0], cube([0.06, bar_w, 0.07])), rack)
    for s in (-1, 1):
        m.add(translate([x - 0.06, s * (bar_w / 2 - 0.05) - 0.05, 0], cube([0.12, 0.10, 0.10])), rack)
skis = [[0.85, 0.15, 0.12], [0.15, 0.40, 0.85], [0.95, 0.85, 0.20]]
for i in range(3):
    for d in (-0.05, 0.05):
        p = [-0.85, (i - 1) * 0.42 + d - 0.035, 0.07]
        m.add([translate(p, cube([1.62, 0.07, 0.025])),
               translate(p, translate([1.62, 0, 0], rotate([0, -25, 0], cube([0.12, 0.07, 0.02]))))],  # tip
              skis[i])
# Clamps over the skis.
for x in (-0.45, 0.45):
    m.add(translate([x - 0.04, -bar_w / 2 + 0.1, 0.095], cube([0.08, bar_w - 0.2, 0.03])), rack)
m.main()

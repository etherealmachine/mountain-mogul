"""Roof box — a long streamlined cargo pod on crossbars, set on a car's
roof (render.carRoofs gives each kind's roof height). Origin at its
centre on the roof. Near white, tinted per car (black, grey, silver or
white).

The pod is a hull of slabs with their corners rounded in plan, so it
reads as a moulded shell rather than a crate.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.car_kit import hull_of, ring
from lib.mogul import *

m = Model()
for x in (-0.45, 0.45):
    m.add(translate([x - 0.03, -0.70, 0], cube([0.06, 1.40, 0.07])), [0.10, 0.10, 0.10])
m.add(hull_of(
    ring(-0.90, 0.65, 0.80, 0.07, 0.20, seg=2),
    ring(-0.90, 0.65, 0.76, 0.17, 0.20, seg=2),
    ring(-0.80, 0.50, 0.72, 0.40, 0.18, seg=2),
    ring(-0.74, 0.44, 0.62, 0.45, 0.16, seg=2),
    ring(0.95, 0.99, 0.36, 0.10, 0.02, seg=1),
    ring(0.95, 0.99, 0.36, 0.24, 0.02, seg=1),
), [0.92, 0.92, 0.92])
m.main()

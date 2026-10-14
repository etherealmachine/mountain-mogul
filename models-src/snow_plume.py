"""Snow plume — the cloud of snow a snow gun blows while it's making
snow (snow_gun.py), drawn see-through over the run.

It leaves the nozzle (about 1.3 m up, just past the barrel's tip on +X),
rises in a fan as it carries, and settles toward the snow 14 m out,
spreading from a hand's width to about 4 m across. A chain of hulled
puffs keeps it soft and cheap.

Conventions, units, and axis docs: see models-src/README.md.
"""

import math

from lib.mogul import *

nozzle = (1.3, 1.33)  # x, z of snow_gun.py's nozzle
length = 14.0
puffs = 9

pts = []
for i in range(puffs):
    f = i / (puffs - 1)
    x = nozzle[0] + f * length
    z = nozzle[1] + 2.6 * math.sin(math.pi * f * 0.85) - 0.9 * f
    r = 0.18 + 1.9 * f ** 0.8
    pts.append((x, z, r))

m = Model()
for (x0, z0, r0), (x1, z1, r1) in zip(pts, pts[1:]):
    m.add(hull(translate([x0, 0, z0], sphere(r=r0, fn=10)),
               translate([x1, 0, z1], sphere(r=r1, fn=10))), [0.96, 0.97, 1.0])
m.main()

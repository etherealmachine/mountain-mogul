"""Snow block — one tumbled block of avalanche debris (render/avalanche.go
scatters them over Cell.Debris, turned, tilted and scaled). A chunky,
faceted lump about 1 m across, 0.8 m deep and 0.6 m tall at scale 1, its
base at the origin so it can be sunk into the snow.

Conventions, units, and axis docs: see models-src/README.md.
"""

import random

from lib.mogul import *

snow = [0.93, 0.94, 0.97]

rng = random.Random(7)
pts = []
for i in range(16):
    # Points round a squashed ball, pushed in and out so the hull has
    # broken, uneven faces.
    u = rng.uniform(-1, 1)
    a = rng.uniform(0, 6.2832)
    r = (1 - u * u) ** 0.5
    k = rng.uniform(0.75, 1.0)
    pts.append((0.5 * k * r * math.cos(a), 0.4 * k * r * math.sin(a), 0.3 + 0.3 * k * u))
# A flat-ish base to sit on.
for a in range(5):
    t = a * 6.2832 / 5 + 0.3
    pts.append((0.38 * math.cos(t), 0.3 * math.sin(t), 0.0))

m = Model()
m.add(hull_points(pts), snow)
m.main()

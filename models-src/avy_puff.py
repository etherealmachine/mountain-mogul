"""Avalanche puff — one billow of an avalanche's powder cloud, or of a
control round's blast and smoke (render/avalanche.go), drawn see-through
and scaled as it grows. A soft lump of overlapping balls, about 1 m
across at scale 1, centred on the origin.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

white = [0.96, 0.97, 1.0]

m = Model()
m.add(sphere(r=0.5, fn=10), white)
m.add(translate([0.28, 0.12, 0.08], sphere(r=0.38, fn=8)), white)
m.add(translate([-0.25, -0.15, 0.1], sphere(r=0.36, fn=8)), white)
m.add(translate([0.05, 0.22, -0.12], sphere(r=0.34, fn=8)), white)
m.add(translate([-0.08, -0.05, 0.3], sphere(r=0.32, fn=8)), white)
m.main()

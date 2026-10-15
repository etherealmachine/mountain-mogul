"""Steam puff — one puff of a steam engine's steam (steam_loco.py's
chimney), drawn see-through and scaled as it rises and spreads
(render/trains.go). A soft lump of a few overlapping balls, about 1 m
across at scale 1, centred on the origin.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

white = [0.94, 0.94, 0.96]

m = Model()
m.add(sphere(r=0.5, fn=10), white)
m.add(translate([0.25, 0.15, 0.1], sphere(r=0.36, fn=8)), white)
m.add(translate([-0.22, -0.18, 0.12], sphere(r=0.34, fn=8)), white)
m.add(translate([0.05, -0.1, 0.32], sphere(r=0.3, fn=8)), white)
m.main()

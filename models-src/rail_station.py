"""Railway station — a small mountain station house (rail_kit.py's
axes): a timber upper storey on a stone ground floor, a deep pitched
roof, and a canopy on posts out over the platform (-Y, toward the
track). About 14 × 7 m; the platform itself is drawn along the track
(render/rail_mesh.go).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

stone = [0.62, 0.60, 0.56]
timber = [0.50, 0.33, 0.20]
roof = [0.32, 0.20, 0.16]
glass = [0.20, 0.26, 0.32]
cream = [0.90, 0.86, 0.74]

m = Model()
m.add(translate([-7, -3.5, 0], cube([14, 7, 3.2])), stone)
m.add(translate([-7, -3.5, 3.2], cube([14, 7, 2.8])), timber)
m.add(translate([0, 0, 6.0], gable(15.2, 8.6, 3.0, 0.15)), roof)
# Windows and doors along the platform side.
for x in (-5.0, -2.5, 2.5, 5.0):
    m.add(translate([x - 0.6, -3.55, 1.0], cube([1.2, 0.06, 1.5])), glass)
    m.add(translate([x - 0.6, -3.55, 4.0], cube([1.2, 0.06, 1.2])), glass)
m.add(translate([-0.9, -3.55, 0], cube([1.8, 0.06, 2.6])), [0.30, 0.20, 0.12])
# The name board over the door.
m.add(translate([-2.0, -3.6, 2.75], cube([4.0, 0.06, 0.4])), cream)
# Canopy over the platform on posts.
m.add(translate([-9, -9.5, 3.4], cube([18, 6.0, 0.25])), roof)
for x in (-8, -3, 3, 8):
    m.add(translate([x, -9.0, 0], cylinder(h=3.4, r=0.12, fn=8)), [0.25, 0.25, 0.27])
m.footprint(7.0, 3.5)
m.main()

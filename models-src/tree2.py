"""Conifer variant 1 — tall slender. Narrower canopy on a slightly taller
trunk; reads as the "lodgepole pine" of the three variants. Drawn by the
thousand: kept faceted (fn=6), as the OpenSCAD source's $fn = 6 was.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

trunk_r = 0.15
trunk_h = 1.80
canopy_r = 1.30
canopy_h = 6.00

m = Model()
m.add(cylinder(h=trunk_h, r=trunk_r, fn=6), "SaddleBrown")
# Foliage: dark spruce green — slightly cooler / darker than tree.py.
m.add(translate([0, 0, trunk_h], cylinder(h=canopy_h, r1=canopy_r, r2=0, fn=6)), [0.22, 0.40, 0.22])
m.main()

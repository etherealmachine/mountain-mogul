"""Conifer variant 2 — christmas-tree double cone. Lower truncated frustum
stacked under an upper full cone; gives the forest one obviously-
different silhouette so dense stands don't look like a single repeated
stamp. Drawn by the thousand: kept faceted (fn=6), as the OpenSCAD
source's $fn = 6 was.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

trunk_r = 0.18
trunk_h = 1.50
lower_r = 1.70
lower_top_r = 0.90
lower_h = 2.40
upper_r = 0.90
upper_h = 2.80

# Foliage: blue-green fir — distinct from tree.py / tree2.py.
foliage = [0.30, 0.48, 0.36]

m = Model()
m.add(cylinder(h=trunk_h, r=trunk_r, fn=6), "SaddleBrown")
# Both tiers in one solid: the hidden faces where they meet drop out.
lower = translate([0, 0, trunk_h], cylinder(h=lower_h, r1=lower_r, r2=lower_top_r, fn=6))
upper = translate([0, 0, trunk_h + lower_h], cylinder(h=upper_h, r1=upper_r, r2=0, fn=6))
m.add(union(lower, upper), foliage)
m.main()

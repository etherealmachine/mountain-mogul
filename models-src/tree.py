"""Conifer variant 0 — small bushy. Short trunk + a single fat hexagonal
cone for the canopy. One of three conifer variants the auto-forest pulls
from based on the per-cell hash; per-instance ColorTint supplies the
foliage shade at draw time, so the white-texture path is fine. Drawn by
the thousand: kept faceted (fn=6).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

trunk_r = 0.18
trunk_h = 1.30
canopy_r = 1.60
canopy_h = 4.50

m = Model()
# Trunk — short stub; mostly hidden by the canopy at gameplay zoom.
m.add(cylinder(h=trunk_h, r=trunk_r, fn=6), "SaddleBrown")
# Canopy — single hex pyramid, base sat on the trunk top. Foliage colour:
# medium pine green. Per-variant differences in this file vs. tree2/tree3
# carry both shape and palette variation so a forest reads as a species
# mix rather than a recoloured stamp.
m.add(translate([0, 0, trunk_h], cylinder(h=canopy_h, r1=canopy_r, r2=0, fn=6)), [0.32, 0.50, 0.28])
m.main()

"""T-bar tower — one of the towers carrying a T-bar's cable: a steel pole
on a footing, a crossarm across the top, and a pair of sheaves at each
end with the cable (world.TBarCableHeight, world.CableGap either side)
running under them. The hangers' spring boxes pass under the sheaves.

+X runs up the line. Conventions, units, and axis docs: see
models-src/README.md.
"""

from lib.mogul import *
from lib.tbar_kit import CABLE_H, CABLE_GAP

sheave_r = 0.14
arm_z = CABLE_H + 2 * sheave_r + 0.12

m = Model()
m.add(translate([0, 0, 0.15], cylinder(0.5, r=0.45, center=True, fn=10)), "Gray")
m.add(cylinder(arm_z + 0.15, r1=0.2, r2=0.14, fn=10), "DarkSlateGray")
m.add(translate([0, 0, arm_z + 0.1], cube([0.2, 2 * CABLE_GAP + 0.6, 0.2], center=True)), "DarkSlateGray")
for y in (-CABLE_GAP, CABLE_GAP):
    m.add(translate([0, y, (arm_z + CABLE_H + sheave_r) / 2], cube([0.06, 0.06, arm_z - CABLE_H - sheave_r], center=True)), "DarkSlateGray")
    m.add(translate([0, y, CABLE_H + sheave_r], cube([0.7, 0.05, 0.05], center=True)), "DarkSlateGray")
    for x in (-0.25, 0.25):
        m.add(translate([x, y, CABLE_H + sheave_r], rotate([90, 0, 0], cylinder(0.06, r=sheave_r, center=True, fn=10))), "Gold")
m.main()

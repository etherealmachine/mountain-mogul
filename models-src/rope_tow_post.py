"""Rope tow post — one of the posts holding a rope tow's rope up between
its ends (world.Lift.RopePostXZs): a post on the line with a crossarm
across it (Y), a pulley under each end for the up and return ropes,
world.RopeGap off the line at world.RopeHeight.

+X runs up the line. Conventions, units, and axis docs: see
models-src/README.md.
"""

from lib.mogul import *
from lib.rope_tow_kit import ROPE_GAP, ROPE_HEIGHT

arm_z = ROPE_HEIGHT + 0.12

m = Model()
m.add(translate([0, 0, (arm_z + 0.1) / 2 - 0.3], cube([0.1, 0.1, arm_z + 0.4], center=True)), "DarkSlateGray")
m.add(translate([0, 0, arm_z], cube([0.08, 2 * ROPE_GAP + 0.3, 0.08], center=True)), "DarkSlateGray")
for y in (-ROPE_GAP, ROPE_GAP):
    m.add(translate([0, y, ROPE_HEIGHT], rotate([90, 0, 0], cylinder(0.04, r=0.1, center=True, fn=10))), "Gold")
    m.add(translate([0, y, (arm_z + ROPE_HEIGHT) / 2], cube([0.03, 0.03, arm_z - ROPE_HEIGHT], center=True)), "DarkSlateGray")
m.main()

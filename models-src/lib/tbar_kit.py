"""Parts shared by a T-bar's models: its ends (tbar_base.py, tbar_top.py),
towers (tbar_tower.py) and hangers (tbar_hanger.py, tbar_hanger_up.py).

The cable runs CABLE_H over the snow the whole way (world.TBarCableHeight),
CABLE_GAP off the line each side (world.CableGap). Riders stand on the snow
under the up cable, two to a bar either side of its shank. Each end is
placed facing the other (+X up the line), so the riders are on -Y at the
base and +Y at the top. Keep in step with world.tbarStation.
"""

from lib.mogul import *
from lib.rope_tow_kit import add_control_post

CABLE_H = 4.5  # world.TBarCableHeight
CABLE_GAP = 1.5  # world.CableGap
BAR_Z = 0.75  # the crossbar over the snow: behind a rider's thighs
BAR_HALF = 0.55
RIDER_Y = 0.45  # each rider either side of the shank
MAST_X = -2.2  # the mast behind the bullwheel


def add_grip(m):
    """The spring box clamped to the cable at the origin, its line reeled
    out below."""
    m.add(translate([0, 0, -0.12], cube([0.12, 0.06, 0.12], center=True)), "DimGray")
    m.add(translate([0, 0, -0.42], cube([0.34, 0.16, 0.36], center=True)), "Gold")


def add_bar(m, bar_z, shank=1.6):
    """The shank, shank long, and the T, its crossbar at bar_z (below the
    cable)."""
    m.add(translate([0, 0, bar_z + shank / 2], cube([0.05, 0.05, shank], center=True)), "Silver")
    m.add(translate([0, 0, bar_z], cube([0.07, 2 * BAR_HALF, 0.07], center=True)), "Firebrick")


def add_tbar_end(m, rider_side, base):
    """A T-bar end with riders on rider_side (-1 for -Y, +1 for +Y): the
    bullwheel flat at cable height on a boom off a mast behind it, and a
    control post outside the riders (world.tbarStation's operator spot
    just inside it). The base's mast carries the motor; the top's, the
    tension weight."""
    wheel_z = CABLE_H - 0.06
    m.add(translate([0, 0, wheel_z], cylinder(0.12, r=CABLE_GAP, fn=24)), "Firebrick")
    m.add(translate([0, 0, wheel_z - 0.05], cylinder(0.25, r=0.22, fn=10)), "Gold")
    top_z = CABLE_H + 0.9
    m.add(translate([MAST_X, 0, top_z / 2], cube([0.4, 0.4, top_z], center=True)), "DarkSlateGray")
    m.add(translate([MAST_X / 2, 0, CABLE_H + 0.4], cube([-MAST_X + 0.2, 0.3, 0.3], center=True)), "DarkSlateGray")
    m.add(translate([0, 0, CABLE_H + 0.2], cube([0.2, 0.2, 0.4], center=True)), "DarkSlateGray")
    m.add(translate([MAST_X, 0, 0.15], cube([1.2, 1.2, 0.3], center=True)), "Gray")
    if base:
        m.add(translate([MAST_X - 0.2, 0, top_z + 0.35], cube([1.1, 0.9, 0.7], center=True)), "SteelBlue")
    else:
        m.add(translate([MAST_X - 0.45, 0, 1.6], cube([0.5, 0.5, 0.9], center=True)), "Gray")
        m.add(translate([MAST_X - 0.45, 0, 2.6], cube([0.03, 0.03, 1.1], center=True)), "DimGray")
    add_control_post(m, 2.9, rider_side * 3.6, -rider_side)

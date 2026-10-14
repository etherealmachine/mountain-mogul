"""Parts shared by a rope tow's two ends (rope_tow_base.py, rope_tow_top.py).

Each end is placed facing the other end (+X up the line). The bullwheel
lies flat at the origin, the rope round it ROPE_GAP off the line each side
(world.RopeGap) at ROPE_HEIGHT (world.RopeHeight). Riders ride outside the
up rope, world.CableGap off the line: on -Y at the base and +Y at the top.
Keep in step with world.ropeStation.
"""

from lib.mogul import *

ROPE_GAP = 1.05  # world.RopeGap
ROPE_HEIGHT = 0.95  # world.RopeHeight
RIDER_GAP = 1.5  # world.CableGap

WHEEL_R = ROPE_GAP
WHEEL_T = 0.12


def add_bullwheel(m):
    """The bullwheel on its frame: a flat wheel the rope runs round, on a
    post braced to a skid on the snow."""
    rim_z = ROPE_HEIGHT - WHEEL_T / 2
    m.add(translate([0, 0, rim_z], cylinder(WHEEL_T, r=WHEEL_R, fn=20)), "Firebrick")
    m.add(translate([0, 0, rim_z - 0.04], cylinder(WHEEL_T + 0.08, r=0.18, fn=10)), "Gold")
    m.add(translate([0, 0, rim_z / 2], cube([0.16, 0.16, rim_z], center=True)), "DarkSlateGray")
    m.add(translate([0, 0, 0.05], cube([1.2, 0.5, 0.1], center=True)), "DarkSlateGray")
    for x in (-0.45, 0.45):
        m.add(hull(
            translate([x, 0, 0.1], cube([0.06, 0.06, 0.02], center=True)),
            translate([0, 0, rim_z - 0.2], cube([0.06, 0.06, 0.02], center=True)),
        ), "DarkSlateGray")


def add_control_post(m, x, y, face):
    """A post with a yellow control box at x, y, its red stop and green
    start buttons facing face (+1 for +Y, -1 for -Y)."""
    m.add(translate([x, y, 0.55], cube([0.07, 0.07, 1.1], center=True)), "DarkSlateGray")
    m.add(translate([x, y + face * 0.02, 1.15], cube([0.2, 0.14, 0.26], center=True)), "Gold")
    for dz, colour in ((0.06, "Red"), (-0.05, "LimeGreen")):
        m.add(translate([x, y + face * 0.1, 1.15 + dz], cube([0.07, 0.04, 0.07], center=True)), colour)


def add_rope_end(m, rider_side, base):
    """A rope tow end with riders on rider_side (-1 for -Y, +1 for +Y). The
    base adds the motor box behind the wheel and the operator's control
    post outside the riders, by where they take hold (world.ropeStation's
    operator spot is just inside it); the top, a stop post outside the
    riders, past where they let go."""
    add_bullwheel(m)
    if base:
        # The motor, behind the wheel, its shaft up to the hub.
        m.add(translate([-1.75, 0, 0.4], cube([0.9, 0.7, 0.8], center=True)), "SteelBlue")
        m.add(translate([-1.75, 0, 0.82], cube([0.95, 0.75, 0.05], center=True)), "DimGray")
        m.add(hull(
            translate([-1.3, 0, 0.6], cube([0.05, 0.08, 0.08], center=True)),
            translate([0, 0, ROPE_HEIGHT - 0.12], cube([0.05, 0.08, 0.08], center=True)),
        ), "DimGray")
        add_control_post(m, 1.6, rider_side * 3.0, -rider_side)
    else:
        add_control_post(m, -0.3, rider_side * 2.7, -rider_side)

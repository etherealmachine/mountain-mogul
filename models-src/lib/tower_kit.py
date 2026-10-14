"""Tower kit — the chairlift tower (tower.py, the 8-pack's tower_wide.py,
funitel_tower.py): a tapered round steel pole on a concrete footing, a
crossarm across the top, a sheave train under it for each cable, and a
maintenance ladder up the back of the pole. Drawn dozens of times per
lift, so kept faceted with fn=.

The cables cross the tower at cable_h (world.TowerHeight, or the lift
type's line height), at the given offsets across the line. The crossarm
sits above them, the sheave wheels hang from it with the cable running
along their undersides, and the pole narrows to clear the chairs' inner
edge.
"""

from lib.mogul import *


def chair_tower(m, ropes=(1.5, -1.5), cable_h=18.0):
    """Add a tower to m carrying a cable at each offset in ropes, cable_h
    over the ground."""
    cable_gap = max(abs(y) for y in ropes)

    # ── Dimensions (metres) ────────────────────────────────────────────────

    pole_r0 = 0.42  # at the ground
    pole_r1 = 0.24  # at the crossarm
    footing_r = 0.80
    footing_h = 0.45  # above ground; it carries on below for sloped ground

    sheave_r = 0.18
    sheave_t = 0.10
    sheaves = 4  # per train
    sheave_dx = 0.55  # spacing along the cable
    train_len = sheaves * sheave_dx + 0.3

    arm_z = cable_h + 2 * sheave_r + 0.22  # crossarm underside
    arm_h = 0.34
    arm_w = 0.30  # along the cable
    arm_half = cable_gap + 0.75

    ladder_x = -(pole_r1 + 0.22)  # up the back of the pole
    ladder_w = 0.40
    ladder_z0 = 2.4  # out of reach from the ground
    rung_dz = 0.6


    # ── Footing and pole ───────────────────────────────────────────────────
    # Footing, with a bevelled top edge.
    m.add(union(translate([0, 0, -1.5], cylinder(h=1.5 + footing_h - 0.08, r=footing_r, fn=20)),
                translate([0, 0, footing_h - 0.08], cylinder(h=0.08, r1=footing_r, r2=footing_r - 0.08, fn=20))),
          "Silver")
    m.add(cylinder(h=arm_z + arm_h, r1=pole_r0, r2=pole_r1, fn=28), "SlateGray")

    # ── Crossarm ───────────────────────────────────────────────────────────
    # Deeper at the pole, tapering to its ends.
    arm = [(arm_half, arm_h), (arm_half, arm_h * 0.35), (pole_r1 + 0.3, 0)]
    arm = arm + [(-y, z) for y, z in reversed(arm)]
    # The profile is drawn across the cable (Y) and up, and extruded along X.
    m.add(translate([-arm_w / 2, 0, arm_z], rotate([90, 0, 90], linear_extrude(arm, arm_w))), "SlateGray")
    # A cap over the pole head.
    m.add(translate([0, 0, arm_z + arm_h], cylinder(h=0.12, r1=pole_r1 + 0.08, r2=pole_r1 + 0.04, fn=20)), "DimGray")


    # ── Sheave trains ──────────────────────────────────────────────────────
    # Wheels in a row along the cable, their bottoms on the cable, in a frame
    # hung from the crossarm.
    def sheave_train(y):
        wheel_z = cable_h + sheave_r
        m.add([translate([(i - (sheaves - 1) / 2) * sheave_dx, y, wheel_z],
                         rotate([90, 0, 0], cylinder(h=sheave_t, r=sheave_r, center=True, fn=14)))
               for i in range(sheaves)], "DarkSlateGray")
        # Side plates over the axles, and the hanger to the crossarm.
        parts = [translate([0, y + s * (sheave_t / 2 + 0.04), wheel_z + 0.06], cube([train_len, 0.04, 0.16], center=True))
                 for s in (-1, 1)]
        parts.append(translate([0, y, (wheel_z + arm_z) / 2 + 0.04], cube([0.22, 0.22, arm_z - wheel_z], center=True)))
        m.add(parts, "Goldenrod")


    for y in ropes:
        sheave_train(y)

    # ── Ladder ─────────────────────────────────────────────────────────────
    ladder = [translate([ladder_x, s * ladder_w / 2, (ladder_z0 + arm_z) / 2],
                        cube([0.05, 0.05, arm_z - ladder_z0], center=True)) for s in (-1, 1)]
    z = ladder_z0 + 0.3
    while z <= arm_z - 0.2 + 1e-9:
        ladder.append(translate([ladder_x, 0, z], cube([0.04, ladder_w, 0.04], center=True)))
        z += rung_dz
    # Stand-offs tying the rails to the pole.
    for z in (ladder_z0 + 1, (ladder_z0 + arm_z) / 2, arm_z - 1):
        ladder.append(translate([(ladder_x - pole_r1) / 2, 0, z],
                                cube([abs(ladder_x) - pole_r1 + 0.1, 0.05, 0.05], center=True)))
    m.add(ladder, "DimGray")

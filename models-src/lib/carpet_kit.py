"""Parts shared by the magic carpet's two ends (magic_carpet_end.py at the
base, magic_carpet_top.py at the top).

Each end is placed facing the other end (+X up the belt), so the belt,
which runs on one side of the lift's line only, lies on -Y at the base and
on +Y at the top. Keep in step with world.beltStation and
render.generateBeltMesh.
"""

from lib.mogul import *

BELT_GAP = 1.5  # world.CableGap: the belt's centre off the line
BELT_HALF = 0.55  # out to the side walls' outer face (generateBeltMesh)
BELT_TOP = 0.12  # world.BeltHeight
PLATE_BACK = -0.7  # where the ramp meets the snow


def add_carpet_end(m, belt_side):
    """Add a carpet end to m with the belt on belt_side (-1 for -Y, +1 for
    +Y): the ramped end plate over the roller, the comb edge, the side
    walls' end caps, and the post with the stop/start buttons on the line
    side of the belt, where the operator stands at the base."""
    belt_y = belt_side * BELT_GAP
    # The end plate: a ramp from the snow up to the belt, over the roller.
    m.add(hull(
        translate([PLATE_BACK, belt_y, 0.01], cube([0.02, 2 * BELT_HALF, 0.02], center=True)),
        translate([0.0, belt_y, (BELT_TOP + 0.04) / 2 - 0.15], cube([0.12, 2 * BELT_HALF, BELT_TOP + 0.04 + 0.3], center=True)),
    ), "DimGray")
    # Yellow comb edge where the belt meets the plate.
    m.add(translate([-0.03, belt_y, BELT_TOP + 0.035], cube([0.08, 2 * BELT_HALF - 0.1, 0.02], center=True)), "Gold")
    # The side walls' end caps.
    for y in (belt_y - BELT_HALF + 0.05, belt_y + BELT_HALF - 0.05):
        m.add(translate([0.15, y, 0.1], cube([0.5, 0.1, 0.4], center=True)), [0.15, 0.15, 0.15])

    # The control post, a step back from the plate on the line side, its
    # box facing the belt: yellow, with a red stop and a green start.
    post_x = PLATE_BACK - 0.2
    post_y = belt_y - belt_side * (BELT_HALF + 0.35)
    face = belt_side  # toward the belt
    m.add(translate([post_x, post_y, 0.55], cube([0.07, 0.07, 1.1], center=True)), "DarkSlateGray")
    m.add(translate([post_x, post_y + face * 0.02, 1.15], cube([0.2, 0.14, 0.26], center=True)), "Gold")
    for dz, colour in ((0.06, "Red"), (-0.05, "LimeGreen")):
        m.add(translate([post_x, post_y + face * 0.1, 1.15 + dz], cube([0.07, 0.04, 0.07], center=True)), colour)

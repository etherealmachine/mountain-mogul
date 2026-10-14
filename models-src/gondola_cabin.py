"""Monocable detachable (MDG) gondola cabin — 8 passengers in two rows of
four, after a Doppelmayr Omega.

Conventions, units, and axis docs: see models-src/README.md.

    +X = forward (direction of travel along the cable)
    +Y = lateral (perpendicular to cable)
    +Z = up (the writer turns it to the game's Y-up)

Origin sits at the cable-grip point. The entire cabin hangs in -Z: on top
the grip and its carriage, whose rollers run on the terminal's rail; a
hanger arm down to the roof; the body below. The cabin hangs long enough
that, at a terminal (cable at world.BullwheelHeight), its floor is just
over the snow and riders step straight in.

The body is one rounded shell, square in plan with round corners,
that narrows toward the floor like a tub: a red skirt, a wraparound band
of tinted glass, a white roof. Sliding doors on the -Y side (the outside
of the loop, where riders get on and off); ski holders on both ends.
The game tints the whole cabin a touch cooler when it carries riders.
"""

import numpy as np

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
cable_h = 3.65  # world.BullwheelHeight
floor_gap = 0.12  # the floor's underside over the snow at a terminal

cabin_w = 2.10  # lateral (Y) — square in plan, 4 riders side by side at 0.48 m, shoulder to shoulder
cabin_d = 2.10  # along-cable (X) — two facing benches
cabin_h = 2.05  # floor to roof
corner_r = 0.45  # plan corners, rounded
taper = 0.10  # how much the plan draws in at the floor
roof_r = 0.16  # the roof edge, rolled over

# Z anchors (everything hangs in -Z).
cabin_bot_z = -cable_h + floor_gap  # -3.53
cabin_top_z = cabin_bot_z + cabin_h  # -1.48
skirt_top_z = cabin_bot_z + 0.62  # glass from here...
glass_top_z = cabin_top_z - 0.30  # ...to here, under the roof band
hanger_bot_z = cabin_top_z + 0.02

red = [0.74, 0.11, 0.13]
shell = [0.95, 0.95, 0.96]
glass = [0.26, 0.36, 0.44]
dark = [0.18, 0.19, 0.21]
steel = [0.55, 0.57, 0.60]

m = Model()


def plan_at(inset, z, r=corner_r):
    return [(x, y, z) for x, y in rounded_rect(cabin_d - 2 * inset, cabin_w - 2 * inset, r - inset, seg=3)]


# ── Body ───────────────────────────────────────────────────────────────
# Kept faceted (the game draws one per cabin on every gondola): hulls of
# rounded plans, drawn in at the floor, the roof edge rolled in two steps.
walls_top = cabin_top_z - roof_r
tub = hull_points(np.array(plan_at(taper, cabin_bot_z) + plan_at(taper * 0.3, cabin_bot_z + 0.35)
                           + plan_at(0, skirt_top_z)))
walls = translate([0, 0, skirt_top_z - 0.001], linear_extrude(rounded_rect(cabin_d, cabin_w, corner_r, seg=3),
                                                             walls_top - skirt_top_z + 0.002))
roof = hull_points(np.array(plan_at(0, walls_top) + plan_at(roof_r * 0.35, cabin_top_z - roof_r * 0.3)
                            + plan_at(roof_r, cabin_top_z)))
m.add(tub, red, tolerance=0.02)
band = translate([0, 0, (skirt_top_z + glass_top_z) / 2],
                 cube([cabin_d + 0.2, cabin_w + 0.2, glass_top_z - skirt_top_z], center=True))
m.add(walls & band, glass, tolerance=0.02)
m.add([walls - band, roof], shell, tolerance=0.02)

# Pillars between the panes: one each side of the door and at the middle
# of the ends, standing just proud of the glass.
pil = 0.07
pillars = [translate([x, -cabin_w / 2 - 0.005, (skirt_top_z + glass_top_z) / 2],
                     cube([pil, 0.04, glass_top_z - skirt_top_z], center=True)) for x in (-0.45, 0.45)]
pillars += [translate([0, s * (cabin_w / 2 - 0.005), (skirt_top_z + glass_top_z) / 2],
                      cube([pil, 0.04, glass_top_z - skirt_top_z], center=True)) for s in (1,)]
pillars += [translate([s * (cabin_d / 2 - 0.005), y, (skirt_top_z + glass_top_z) / 2],
                      cube([0.04, pil, glass_top_z - skirt_top_z], center=True))
            for s in (-1, 1) for y in (-0.3, 0.3)]
m.add(pillars, shell)

# Doors: two sliding leaves on the -Y side, the seam and edges dark.
door_w, door_z0, door_z1 = 0.88, cabin_bot_z + 0.10, glass_top_z + 0.12
dy = -cabin_w / 2 - 0.012
m.add([translate([x, dy, (door_z0 + door_z1) / 2], cube([0.025, 0.03, door_z1 - door_z0], center=True))
       for x in (-door_w / 2, 0, door_w / 2)]
      + [translate([0, dy, door_z1], cube([door_w, 0.03, 0.025], center=True))], dark)
# The skirt's door leaves in white, so the door reads from afar.
m.add([translate([s * door_w / 4, dy + 0.01, (door_z0 + skirt_top_z) / 2],
                 cube([door_w / 2 - 0.04, 0.02, skirt_top_z - door_z0], center=True)) for s in (-1, 1)], shell)

# Ski holders on both ends: a dark frame with a few skis stood up in it.
for s in (-1, 1):
    x = s * (cabin_d / 2 + 0.06)
    m.add([translate([x, 0, skirt_top_z - 0.05], cube([0.10, 1.2, 0.06], center=True)),
           translate([x, 0, glass_top_z - 0.05], cube([0.10, 1.2, 0.05], center=True))], dark)
    skis = [(-0.48, [0.85, 0.75, 0.12]), (-0.36, [0.85, 0.75, 0.12]), (0.34, [0.15, 0.55, 0.75]),
            (0.46, [0.15, 0.55, 0.75])]
    for y, c in skis:
        m.add(translate([x + s * 0.05, y, (cabin_bot_z + 0.3 + glass_top_z + 0.15) / 2],
                        cube([0.025, 0.08, glass_top_z - cabin_bot_z - 0.15], center=True)), c)

# Floor pan.
m.add(translate([0, 0, cabin_bot_z - 0.02], linear_extrude(
    rounded_rect(cabin_d - 2 * taper - 0.04, cabin_w - 2 * taper - 0.04, corner_r - taper, seg=2), 0.03)), dark)

# ── Hanger, carriage and grip ──────────────────────────────────────────
# A roof plate, the hanger arm up to the carriage, and the carriage: a
# beam with a roller at each corner, which run on the terminal's rail,
# and the grip under the cable.
m.add(translate([0, 0, cabin_top_z - 0.01], cylinder(h=0.06, r=0.34, fn=10)), steel)
arm = [rod([0, 0, hanger_bot_z], [0, 0.10, -0.45], 0.065, fn=8),
       rod([0, 0.10, -0.45], [0, 0, -0.18], 0.065, fn=8)]
m.add(arm, steel)
m.add(translate([0, 0, -0.14], cube([0.84, 0.14, 0.12], center=True)), dark)
m.add([translate([x, y, -0.10], rotate([90, 0, 0], cylinder(h=0.05, r=0.07, fn=8, center=True)))
       for x in (-0.36, 0.36) for y in (-0.10, 0.10)], [0.12, 0.12, 0.13])
m.add(translate([0, 0, -0.02], cube([0.30, 0.12, 0.16], center=True)), steel)

# ── Benches and riders ─────────────────────────────────────────────────
# Eight riders seated on two benches facing each other, four on each,
# 0.48 m apart: the rear bench (-X) faces forward, the front faces back.
# Their skis ride in the racks outside. A rider's slot is where their feet
# are; the figure's hips sit 0.2 m back from them, over the bench.
from lib.cabin_kit import bench, seated_row  # noqa: E402

floor_z = cabin_bot_z + 0.03
inner = cabin_d / 2 - 0.08  # the end walls' inside
for s in (-1, 1):
    bench(m, s * inner, s * (inner - 0.40), 0.95, floor_z)
ys = (-0.72, -0.24, 0.24, 0.72)
i = seated_row(m, 0, inner - 0.42, ys, floor_z, back=True)  # front bench
seated_row(m, i, -(inner - 0.42), ys, floor_z, back=False)  # rear bench
m.main()

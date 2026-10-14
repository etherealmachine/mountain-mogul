"""Cabin kit — the bodies of the funitel's and the tricable gondola's
cabins (funitel_cabin.py, tri_cabin.py), in the gondola's style
(gondola_cabin.py): a rounded tub drawn in at the floor, a coloured
skirt, a wraparound band of tinted glass, a white roof, pillars between
the panes, and a double sliding door in the middle of each side across
the line.

    +X = along the line, +Y = across it, +Z = up; the body's floor at
    bot_z, centred on the origin in plan.
"""

import numpy as np

from lib.mogul import *

shell = [0.95, 0.95, 0.96]
glass = [0.26, 0.36, 0.44]
dark = [0.18, 0.19, 0.21]
steel = [0.50, 0.52, 0.56]


def cabin_body(m, length, width, height, bot_z, skirt, corner_r=0.5, taper=0.12, roof_r=0.18,
               door_w=1.2, pillars_x=(), pillars_y=()):
    """Add a cabin body length × width in plan and height tall, its floor
    at bot_z, its skirt coloured skirt. Returns the roof's top z."""
    top_z = bot_z + height
    skirt_top = bot_z + 0.65
    glass_top = top_z - 0.32

    def plan_at(inset, z):
        return [(x, y, z) for x, y in rounded_rect(length - 2 * inset, width - 2 * inset, corner_r - inset, seg=3)]

    walls_top = top_z - roof_r
    tub = hull_points(np.array(plan_at(taper, bot_z) + plan_at(taper * 0.3, bot_z + 0.35) + plan_at(0, skirt_top)))
    walls = translate([0, 0, skirt_top - 0.001], linear_extrude(rounded_rect(length, width, corner_r, seg=3),
                                                               walls_top - skirt_top + 0.002))
    roof = hull_points(np.array(plan_at(0, walls_top) + plan_at(roof_r * 0.35, top_z - roof_r * 0.3)
                                + plan_at(roof_r, top_z)))
    m.add(tub, skirt, tolerance=0.02)
    band = translate([0, 0, (skirt_top + glass_top) / 2],
                     cube([length + 0.2, width + 0.2, glass_top - skirt_top], center=True))
    m.add(walls & band, glass, tolerance=0.02)
    m.add([walls - band, roof], shell, tolerance=0.02)

    gh, gz = glass_top - skirt_top, (skirt_top + glass_top) / 2
    p = [translate([x, s * (width / 2 + 0.005), gz], cube([0.08, 0.04, gh], center=True))
         for s in (-1, 1) for x in pillars_x]
    p += [translate([s * (length / 2 + 0.005), y, gz], cube([0.04, 0.08, gh], center=True))
          for s in (-1, 1) for y in pillars_y]
    if p:
        m.add(p, shell)

    # A double sliding door in the middle of each side across the line.
    z0, z1 = bot_z + 0.1, glass_top + 0.12
    for s in (-1, 1):
        dy = s * (width / 2 + 0.012)
        m.add([translate([x, dy, (z0 + z1) / 2], cube([0.03, 0.03, z1 - z0], center=True))
               for x in (-door_w / 2, 0, door_w / 2)]
              + [translate([0, dy, z1], cube([door_w, 0.03, 0.03], center=True))], dark)
        m.add([translate([k * door_w / 4, dy + s * 0.01, (z0 + skirt_top) / 2],
                         cube([door_w / 2 - 0.05, 0.02, skirt_top - z0], center=True)) for k in (-1, 1)], shell)

    m.add(translate([0, 0, bot_z - 0.02], linear_extrude(
        rounded_rect(length - 2 * taper - 0.04, width - 2 * taper - 0.04, corner_r - taper, seg=2), 0.03)), dark)
    return top_z


bench_color = [0.30, 0.34, 0.42]


def bench(m, x_wall, x_front, y_half, floor_z, seat_h=0.46):
    """A bench along a cabin's end wall, from the wall (x_wall) out to its
    front edge (x_front), y_half either side of the middle, its seat
    seat_h over the floor at floor_z: a seat slab on a plinth."""
    x0, x1 = min(x_wall, x_front), max(x_wall, x_front)
    m.add([translate([(x0 + x1) / 2, 0, floor_z + seat_h - 0.04], cube([x1 - x0, 2 * y_half, 0.08], center=True)),
           translate([(x0 + x1) / 2 + (0.06 if x_wall < x_front else -0.06), 0, floor_z + (seat_h - 0.08) / 2],
                     cube([x1 - x0 - 0.12, 2 * y_half - 0.1, seat_h - 0.08], center=True))], bench_color)


def seated_row(m, first, x_feet, ys, z, back):
    """Slots for riders on a bench, feet at x_feet on the floor at z, one
    at each y: facing the back of the cabin if back. Returns the next
    slot index."""
    for y in ys:
        m.slot(first, x_feet, y, z, "sit-back" if back else "sit")
        first += 1
    return first

"""Aerial tram cabin — 100 riders standing, as Jackson Hole's (100) and
Palisades' (110), in the gondola's livery. Sized from Portland's
Gangloff cabins, 7.62 × 3.35 m for 78 and an operator, about 0.3 m² a
rider inside: 8.6 × 3.5 m.

Conventions, units, and axis docs: see models-src/README.md.

    +X = up the line (a tram cabin never turns: it runs up and back down
         facing the same way)
    +Y = across the line
    +Z = up (the writer turns it to the game's Y-up)

Origin sits on the track rope. On top the carriage, its wheels on the
rope; a heavy hanger arm down to a yoke on the roof; the body below,
hanging far enough that at a terminal (track at world.TramDockHeight)
its floor is just over the snow and riders walk straight in. The body is
a long tub with round ends, a red skirt, a wraparound band of tinted
glass and a white roof with a railed walkway; a double sliding door in
the middle of each long side.
"""

import numpy as np

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
dock_h = 7.0  # world.TramDockHeight
rope_y = 0.4  # world.TramRopeSpread: the two track ropes either side of the origin
floor_gap = 0.15  # the floor's underside over the snow at a terminal

cabin_l = 8.6  # along the line (X)
cabin_w = 3.5  # across (Y): world.TramGap leaves a platform between the docked cabins
cabin_h = 2.65  # floor to roof
end_r = 1.0  # plan corners: the ends round off
taper = 0.14  # how much the plan draws in at the floor
roof_r = 0.20  # the roof edge, rolled over

cabin_bot_z = -dock_h + floor_gap  # -6.85
cabin_top_z = cabin_bot_z + cabin_h  # -4.20
skirt_top_z = cabin_bot_z + 0.75
glass_top_z = cabin_top_z - 0.35

red = [0.74, 0.11, 0.13]
shell = [0.95, 0.95, 0.96]
glass = [0.26, 0.36, 0.44]
dark = [0.18, 0.19, 0.21]
steel = [0.50, 0.52, 0.56]
yellow = [0.92, 0.74, 0.10]

m = Model()


def plan_at(inset, z, r=end_r):
    return [(x, y, z) for x, y in rounded_rect(cabin_l - 2 * inset, cabin_w - 2 * inset, r - inset, seg=4)]


# ── Body ───────────────────────────────────────────────────────────────
walls_top = cabin_top_z - roof_r
tub = hull_points(np.array(plan_at(taper, cabin_bot_z) + plan_at(taper * 0.3, cabin_bot_z + 0.4)
                           + plan_at(0, skirt_top_z)))
walls = translate([0, 0, skirt_top_z - 0.001], linear_extrude(rounded_rect(cabin_l, cabin_w, end_r, seg=4),
                                                             walls_top - skirt_top_z + 0.002))
roof = hull_points(np.array(plan_at(0, walls_top) + plan_at(roof_r * 0.35, cabin_top_z - roof_r * 0.3)
                            + plan_at(roof_r, cabin_top_z)))
m.add(tub, red, tolerance=0.02)
band = translate([0, 0, (skirt_top_z + glass_top_z) / 2],
                 cube([cabin_l + 0.2, cabin_w + 0.2, glass_top_z - skirt_top_z], center=True))
m.add(walls & band, glass, tolerance=0.02)
m.add([walls - band, roof], shell, tolerance=0.02)

# Pillars between the panes, standing just proud of the glass.
gh = glass_top_z - skirt_top_z
gz = (skirt_top_z + glass_top_z) / 2
pillars = [translate([x, s * (cabin_w / 2 + 0.005), gz], cube([0.09, 0.04, gh], center=True))
           for s in (-1, 1) for x in (-2.1, -0.75, 0.75, 2.1)]
m.add(pillars, shell)

# A double sliding door in the middle of each long side: dark edges and
# seam, the skirt's leaves white so the doors read from afar.
door_w, door_z0, door_z1 = 1.5, cabin_bot_z + 0.10, glass_top_z + 0.15
for s in (-1, 1):
    dy = s * (cabin_w / 2 + 0.012)
    m.add([translate([x, dy, (door_z0 + door_z1) / 2], cube([0.03, 0.03, door_z1 - door_z0], center=True))
           for x in (-door_w / 2, 0, door_w / 2)]
          + [translate([0, dy, door_z1], cube([door_w, 0.03, 0.03], center=True))], dark)
    m.add([translate([k * door_w / 4, dy + s * 0.01, (door_z0 + skirt_top_z) / 2],
                     cube([door_w / 2 - 0.05, 0.02, skirt_top_z - door_z0], center=True)) for k in (-1, 1)], shell)

# Floor pan.
m.add(translate([0, 0, cabin_bot_z - 0.02], linear_extrude(
    rounded_rect(cabin_l - 2 * taper - 0.04, cabin_w - 2 * taper - 0.04, end_r - taper, seg=3), 0.03)), dark)

# ── Roof: walkway, railing, equipment ──────────────────────────────────
rail_l, rail_w = cabin_l - 2.2, cabin_w - 0.6
rail_z = cabin_top_z + 0.85
m.add(translate([0, 0, cabin_top_z], cube([rail_l, rail_w, 0.06], center=True)), [0.62, 0.64, 0.66])
rails = [translate([0, s * rail_w / 2, rail_z], cube([rail_l, 0.05, 0.05], center=True)) for s in (-1, 1)]
rails += [translate([s * rail_l / 2, 0, rail_z], cube([0.05, rail_w, 0.05], center=True)) for s in (-1, 1)]
posts = [translate([x, s * rail_w / 2, (cabin_top_z + rail_z) / 2], cube([0.05, 0.05, rail_z - cabin_top_z], center=True))
         for s in (-1, 1) for x in (-rail_l / 2, -rail_l / 6, rail_l / 6, rail_l / 2)]
m.add(rails + posts, yellow)
# Air and drive boxes either side of the yoke.
m.add([translate([s * 1.35, 0, cabin_top_z + 0.22], cube([0.8, 1.2, 0.44], center=True)) for s in (-1, 1)], shell)

# ── Hanger, yoke and carriage ──────────────────────────────────────────
# The yoke across the roof's middle, a tapered arm up to the carriage,
# and the carriage: a long frame with a row of wheels on each track rope.
m.add(translate([0, 0, cabin_top_z + 0.12], cube([0.5, cabin_w - 0.5, 0.24], center=True)), dark)
arm = hull(translate([0, 0, cabin_top_z + 0.3], cube([0.9, 0.36, 0.12], center=True)),
           translate([0, 0, -0.8], cube([0.42, 0.24, 0.1], center=True)))
m.add(arm, steel, tolerance=0.02)
m.add(translate([0, 0, -0.55], cube([3.6, 2 * rope_y + 0.3, 0.5], center=True)), dark)
m.add(translate([0, 0, -0.32], cube([3.2, 2 * rope_y + 0.4, 0.08], center=True)), red)
wheels = [translate([x, s * rope_y + d, -0.13], rotate([90, 0, 0], cylinder(h=0.06, r=0.13, fn=10, center=True)))
          for x in (-1.35, -0.45, 0.45, 1.35) for s in (-1, 1) for d in (-0.08, 0.08)]
m.add(wheels, [0.12, 0.12, 0.13])

# ── Rider slots ────────────────────────────────────────────────────────
# A hundred standing on the floor, six abreast 0.5 m apart, rows 0.42 m
# apart (a rider's 0.48 m shoulders across, 0.36 m front to back), the
# rows at the rounded ends shorter.
slot_z = cabin_bot_z + 0.04
inner_x, inner_y = cabin_l / 2 - 0.12, cabin_w / 2 - 0.12
r_in = end_r - 0.12


def room(x):
    """Half the floor's width at x along it, inside the rounded ends."""
    dx = abs(x) - (inner_x - r_in)
    if dx <= 0:
        return inner_y
    return inner_y - r_in + math.sqrt(max(r_in * r_in - dx * dx, 0))


# Seventeen rows centred on the cabin; the end rows, narrowed by the
# rounded ends, give up their outer places first, then the middle row
# its last, to make a hundred.
rows = [-3.36 + 0.42 * k for k in range(17)]
places = [(x, -1.25 + 0.5 * k) for x in rows for k in range(6)
          if abs(-1.25 + 0.5 * k) + 0.24 <= room(abs(x) + 0.18)]
while len(places) > 100:
    places.remove(max(places, key=lambda p: (abs(p[0]), abs(p[1]))))
for i, (x, y) in enumerate(places):
    m.slot(i, x, y, slot_z, "stand")
m.main()

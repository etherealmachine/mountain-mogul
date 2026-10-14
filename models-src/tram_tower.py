"""Aerial tram tower — a tall steel lattice tower carrying both cabins'
track ropes over saddles at the top of a wide crossarm.

Conventions, units, and axis docs: see models-src/README.md. +X runs
along the line (the renderer turns it to the cable), +Y across.

The origin is the ground at the tower's centre. Four legs splay out at
the foot and close up toward the top, braced on every face; each cabin's
two track ropes run over a pair of saddles world.TramGap either side,
their tops at
world.TramLineHeight, the cable's height over the ground at a tower.
"""

from lib.mogul import *

line_h = 32.0  # world.TramLineHeight
track_y = 3.2  # world.TramGap
rope_y = 0.4  # world.TramRopeSpread: two track ropes per cabin
sink = 1.0
foot = 3.0  # half the legs' spread at the ground
head = 0.9  # and at the top
top_z = line_h - 2.6  # the legs' top, under the crossarm
levels = [0.0, 7.0, 13.5, 19.5, 25.0, top_z]

steel = [0.40, 0.43, 0.47]
dark = [0.20, 0.21, 0.23]
yellow = [0.92, 0.74, 0.10]

m = Model()


def corner(sx, sy, z):
    k = z / top_z
    h = foot + (head - foot) * k
    return [sx * h, sy * h, z]


corners = [(1, 1), (-1, 1), (-1, -1), (1, -1)]
members = []
for sx, sy in corners:
    members.append(rod(corner(sx, sy, -sink), corner(sx, sy, top_z), 0.22, fn=8))
# Horizontal ties at each level and X bracing on each face between them.
for i, z in enumerate(levels[1:], 1):
    zb = levels[i - 1]
    for k in range(4):
        a, b = corners[k], corners[(k + 1) % 4]
        members.append(rod(corner(*a, z), corner(*b, z), 0.09, fn=6))
        members.append(rod(corner(*a, zb), corner(*b, z), 0.07, fn=6))
        members.append(rod(corner(*b, zb), corner(*a, z), 0.07, fn=6))
m.add(members, steel)
# Footings.
m.add([translate(corner(sx, sy, -sink), cylinder(h=sink + 0.5, r=0.6, fn=8)) for sx, sy in corners], "Silver")

# Crossarm across the top, out past both tracks, and the saddles on it.
arm_z = top_z
m.add(translate([0, 0, arm_z + 0.6], cube([1.4, 2 * track_y + 1.6, 1.2], center=True)), steel)
for s in (-1, 1):
    y = s * track_y
    post_h = line_h - 0.6 - (arm_z + 1.2)
    m.add(translate([0, y, arm_z + 1.2 + post_h / 2], cube([0.8, 2 * rope_y + 0.4, post_h], center=True)), steel)
    for r in (-rope_y, rope_y):  # a saddle under each of the cabin's two ropes
        saddle = hull(translate([-2.2, y + r - 0.15, line_h - 0.6], cube([0.1, 0.3, 0.2])),
                      translate([-1.0, y + r - 0.15, line_h - 0.35], cube([2.0, 0.3, 0.35])),
                      translate([2.1, y + r - 0.15, line_h - 0.6], cube([0.1, 0.3, 0.2])))
        m.add(saddle, dark)
# A railed catwalk round the crossarm.
m.add(translate([0, 0, arm_z + 1.25], cube([2.4, 2 * track_y + 1.6, 0.08], center=True)), [0.55, 0.57, 0.6])
m.add([translate([s * 1.2, 0, arm_z + 2.2], cube([0.06, 2 * track_y + 1.6, 0.06], center=True)) for s in (-1, 1)], yellow)
m.add([translate([s * 1.2, y, arm_z + 1.75], cube([0.06, 0.06, 0.95], center=True))
       for s in (-1, 1) for y in (-track_y - 0.7, -1.2, 1.2, track_y + 0.7)], yellow)
m.main()

"""Funitel cabin — 24 riders, twelve seated and twelve standing, wide
across the line, hung from
both of its way's haul ropes by a broad hanger frame: the funitel's wind
stability.

Conventions, units, and axis docs: see models-src/README.md.

    +X = forward (direction of travel), +Y = across, +Z = up.

Riders' skis ride in racks outside (as the gondola's). Origin is
the midpoint between the two haul ropes, which run
world.FunitelRopeSpread either side of it; a grip and carriage on each.
The cabin hangs as the gondola's does (gondola_cabin.py): at a terminal,
the ropes BullwheelHeight over the platform, its floor is just over it.
Blue skirt, the cabin kit's body (lib/cabin_kit.py).
"""

from lib.cabin_kit import cabin_body, dark, steel
from lib.mogul import *

rope_y = 1.6  # world.FunitelRopeSpread
bot_z = -3.65 + 0.12
length, width, height = 2.5, 3.1, 2.3  # tall enough to stand in

m = Model()
top_z = cabin_body(m, length, width, height, bot_z, [0.12, 0.25, 0.48], corner_r=0.45, door_w=1.1,
                   pillars_x=(-0.75, 0.75), pillars_y=(-0.8, 0.8))

# Hanger frame: an arm up from each side of the roof to its rope's grip,
# a cross beam between them, and a carriage with rollers at each grip.
yoke_y = 1.05
m.add(translate([0, 0, top_z + 0.08], cube([0.5, 2 * yoke_y + 0.3, 0.16], center=True)), dark)
m.add([rod([0, s * yoke_y, top_z + 0.1], [0, s * rope_y, -0.2], 0.07, fn=8) for s in (-1, 1)], steel)
m.add(translate([0, 0, -0.75], cube([0.14, 2 * rope_y, 0.12], center=True)), steel)
for s in (-1, 1):
    y = s * rope_y
    m.add(translate([0, y, -0.14], cube([0.8, 0.14, 0.12], center=True)), dark)
    m.add([translate([x, y + d, -0.10], rotate([90, 0, 0], cylinder(h=0.05, r=0.07, fn=8, center=True)))
           for x in (-0.33, 0.33) for d in (-0.1, 0.1)], [0.12, 0.12, 0.13])
    m.add(translate([0, y, -0.02], cube([0.3, 0.12, 0.16], center=True)), steel)

# Riders: six on each end bench facing in (the front one facing back),
# and two rows of six standing between them: half seated, as funitels
# mostly are (Péclet's carry 24, 19 seated). 0.48 m a rider.
from lib.cabin_kit import bench, seated_row  # noqa: E402

floor_z = bot_z + 0.03
inner = length / 2 - 0.08
ys = tuple(-1.2 + 0.48 * k for k in range(6))
for s in (-1, 1):
    bench(m, s * inner, s * (inner - 0.42), 1.44, floor_z)
i = seated_row(m, 0, inner - 0.44, ys, floor_z, back=True)
i = seated_row(m, i, -(inner - 0.44), ys, floor_z, back=False)
for x in (-0.26, 0.26):
    for y in ys:
        m.slot(i, x, y, floor_z, "stand")
        i += 1
m.main()

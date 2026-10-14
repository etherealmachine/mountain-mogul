"""Tricable (3S) gondola cabin — 28 riders, all seated, after Zermatt's
Matterhorn Glacier Ride (28 seats; Peak 2 Peak's carry 28, 24 of them
seated): a floor about 3.7 m square and 2.5 m of headroom inside, as 3S
cabins have. Eight on each end bench, facing in, and six on each side of
a back-to-back bench across the middle; their skis in racks outside. Its
carriage runs on two track ropes, gripped to the haul rope between them.

Conventions, units, and axis docs: see models-src/README.md.

    +X = forward (direction of travel), +Y = across, +Z = up.

Origin is on the haul rope, the track ropes world.TramRopeSpread either
side of it. A long hanger drops to the roof, so at a terminal, the ropes
world.TriDockHeight over the platform, the floor is just over it.
Charcoal skirt, the cabin kit's body (lib/cabin_kit.py).
"""

from lib.cabin_kit import cabin_body, dark, steel
from lib.mogul import *

rope_y = 0.4  # world.TramRopeSpread
dock_h = 5.2  # world.TriDockHeight
bot_z = -dock_h + 0.15
length, width, height = 3.85, 3.85, 2.75

m = Model()
top_z = cabin_body(m, length, width, height, bot_z, [0.22, 0.22, 0.25], corner_r=0.4, door_w=1.0,
                   pillars_x=(-1.1, 1.1), pillars_y=(-0.9, 0.9))
# A red stripe round the skirt.
m.add(translate([0, 0, bot_z + 0.5], linear_extrude(rounded_rect(length + 0.02, width + 0.02, 0.4, seg=3), 0.08)),
      [0.74, 0.11, 0.13])

# Yoke on the roof, the hanger up to the carriage, and the carriage: a
# frame with a row of wheels on each track rope and the grip under it.
m.add(translate([0, 0, top_z + 0.1], cube([0.5, width - 0.6, 0.2], center=True)), dark)
m.add(hull(translate([0, 0, top_z + 0.25], cube([0.8, 0.34, 0.12], center=True)),
           translate([0, 0, -0.75], cube([0.4, 0.22, 0.1], center=True))), steel, tolerance=0.02)
m.add(translate([0, 0, -0.5], cube([2.4, 2 * rope_y + 0.3, 0.42], center=True)), dark)
m.add(translate([0, 0, -0.27], cube([2.0, 2 * rope_y + 0.36, 0.06], center=True)), [0.74, 0.11, 0.13])
m.add([translate([x, s * rope_y + d, -0.12], rotate([90, 0, 0], cylinder(h=0.05, r=0.11, fn=10, center=True)))
       for x in (-0.9, -0.3, 0.3, 0.9) for s in (-1, 1) for d in (-0.07, 0.07)], [0.12, 0.12, 0.13])

# Ski racks on both ends, a frame with a few skis stood up in it.
for s in (-1, 1):
    x = s * (length / 2 + 0.07)
    m.add([translate([x, 0, bot_z + 0.6], cube([0.1, 2.2, 0.06], center=True)),
           translate([x, 0, top_z - 0.45], cube([0.1, 2.2, 0.05], center=True))], dark)
    for y, c in ((-0.9, [0.85, 0.75, 0.12]), (-0.78, [0.85, 0.75, 0.12]), (-0.2, [0.75, 0.2, 0.2]),
                 (-0.08, [0.75, 0.2, 0.2]), (0.6, [0.15, 0.55, 0.75]), (0.72, [0.15, 0.55, 0.75])):
        m.add(translate([x + s * 0.05, y, (bot_z + 0.3 + top_z - 0.3) / 2],
                        cube([0.025, 0.08, top_z - bot_z - 0.6], center=True)), c)

# Riders: eight on each end bench, the front one facing back, and six on
# each side of the middle bench, back to back: its rear side facing back
# toward the rear bench, its front facing forward. 0.45 m a seat (as the
# real 0.46 m), the middle bench short of the side walls for the doors.
from lib.cabin_kit import bench, seated_row  # noqa: E402

floor_z = bot_z + 0.03
inner = length / 2 - 0.08
ends = tuple(-1.575 + 0.45 * k for k in range(8))
mids = tuple(-1.125 + 0.45 * k for k in range(6))
for s in (-1, 1):
    bench(m, s * inner, s * (inner - 0.42), 1.8, floor_z)
    bench(m, 0, s * 0.42, 1.35, floor_z)
i = seated_row(m, 0, inner - 0.44, ends, floor_z, back=True)
i = seated_row(m, i, -(inner - 0.44), ends, floor_z, back=False)
i = seated_row(m, i, -0.42, mids, floor_z, back=True)
seated_row(m, i, 0.42, mids, floor_z, back=False)
m.main()

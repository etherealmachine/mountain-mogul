"""Bulldozer — the picture on the Remove tool's button (a menu thumbnail,
render/thumbnails.go); not placed in the world. A yellow crawler dozer:
two tracks, an engine body, a cab with a roll cage, a curved blade in
front on push arms, and an exhaust stack.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

yellow = [0.96, 0.72, 0.10]
dark = [0.16, 0.16, 0.18]
steel = [0.42, 0.43, 0.46]
glass = [0.30, 0.42, 0.52]

track_l, track_w, track_h = 3.2, 0.55, 0.7
track_y = 0.95  # centre of each track, either side

m = Model(tolerance=0.02)

# Tracks: long boxes with rounded ends, and a few rollers showing.
for s in (-1, 1):
    pts = rounded_rect(track_l, track_h, track_h / 2 - 0.02, seg=4)
    track = rotate([90, 0, 0], linear_extrude(pts, track_w, center=True))
    m.add(translate([0, s * track_y, track_h / 2], track), dark)
    for x in (-1.1, -0.37, 0.37, 1.1):
        m.add(translate([x, s * (track_y + track_w / 2 + 0.01), track_h / 2],
                        rotate([90, 0, 0], cylinder(h=0.04, r=0.18, center=True, fn=10))), steel)

# Engine body and hood, over the tracks.
body = linear_extrude(rounded_rect(2.6, 1.7, 0.12, seg=2), 0.75)
m.add(translate([0.15, 0, track_h - 0.05], body), yellow)
hood = hull(translate([0.95, 0, track_h + 0.7], cube([1.2, 1.5, 0.02], center=True)),
            translate([0.85, 0, track_h + 1.05], cube([1.0, 1.3, 0.02], center=True)))
m.add(hood, yellow)
# Grille on the hood's front.
m.add(translate([1.56, 0, track_h + 0.85], cube([0.04, 1.1, 0.3], center=True)), dark)

# Cab: a box with glass, under a roof on posts.
cab_x, cab_z = -0.55, track_h + 0.7
m.add(translate([cab_x, 0, cab_z + 0.45], cube([1.0, 1.4, 0.9], center=True)), yellow)
m.add([translate([cab_x + 0.51, 0, cab_z + 0.75], cube([0.03, 1.2, 0.55], center=True)),
       translate([cab_x, -0.71, cab_z + 0.75], cube([0.8, 0.03, 0.55], center=True)),
       translate([cab_x, 0.71, cab_z + 0.75], cube([0.8, 0.03, 0.55], center=True))], glass)
m.add(translate([cab_x, 0, cab_z + 1.12], linear_extrude(rounded_rect(1.25, 1.6, 0.1, seg=2), 0.1)), yellow)
for x in (cab_x - 0.55, cab_x + 0.55):
    for y in (-0.72, 0.72):
        m.add(translate([x, y, cab_z], cylinder(h=1.12, r=0.035, fn=6)), dark)

# Exhaust stack.
m.add(translate([0.6, 0.45, track_h + 1.0], cylinder(h=0.6, r=0.07, fn=10)), dark)

# Blade: a curved plate across the front, on push arms.
blade_x = 2.15
arc = []
for k in range(9):
    a = math.radians(-50 + 100 * k / 8)
    arc.append((blade_x - 0.35 * math.cos(a) + 0.35, 0.65 + 0.6 * math.sin(a)))
inner = [(x - 0.07, z) for x, z in reversed(arc)]
profile = arc + inner
blade = rotate([90, 0, 0], linear_extrude([(x, z) for x, z in profile], 2.9, center=True))
m.add(blade, yellow)
m.add(translate([blade_x + 0.28, 0, 0.1], cube([0.12, 2.9, 0.12], center=True)), steel)  # cutting edge
for s in (-1, 1):
    m.add(rod([0.5, s * (track_y + 0.1), 0.45], [blade_x - 0.05, s * 1.05, 0.5], 0.07, fn=8), dark)

m.main()

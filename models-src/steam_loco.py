"""Steam locomotive — an old European tank engine (rail_kit.py): a black
boiler with a tall chimney and brass dome, green side tanks and cab, three
red driving wheels a side joined by coupling rods, buffer beams at both
ends. About 9.6 m over the buffers; the steam style's engine
(notes/next/Trains and Tunnels.md).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

black = [0.08, 0.08, 0.09]
green = [0.10, 0.30, 0.18]
red = [0.62, 0.10, 0.08]
brass = [0.80, 0.62, 0.22]

m = Model()
# Frame and footplate.
m.add(box(-4.2, 4.2, 2.3, 0.85, 1.15), black)
m.add(box(-4.0, 4.0, 2.7, 1.15, 1.22), black)
# Driving wheels: three axles, big, red with dark tyres; coupling rods.
for x in (2.1, 0.45, -1.2):
    for s in (-1, 1):
        m.add(translate([x, s * gauge_half, 0.66], rotate([90, 0, 0], cylinder(h=0.12, r=0.66, center=True, fn=18))), red)
        m.add(translate([x, s * (gauge_half + 0.07), 0.66], rotate([90, 0, 0], cylinder(h=0.03, r=0.2, center=True, fn=8))), black)
for s in (-1, 1):
    m.add(box(-1.25, 2.15, 0.06, 0.52, 0.62, y=s * (gauge_half + 0.13)), grey)
    # Cylinders ahead of the wheels.
    m.add(translate([3.25, s * 1.0, 0.85], rotate([0, 90, 0], cylinder(h=0.9, r=0.3, center=True, fn=12))), black)
# Boiler, smokebox and its door.
m.add(translate([-1.4, 0, 2.15], rotate([0, 90, 0], cylinder(h=4.6, r=0.74, fn=20))), black)
m.add(translate([3.2, 0, 2.15], rotate([0, 90, 0], cylinder(h=0.75, r=0.8, fn=20))), [0.18, 0.18, 0.2])
m.add(translate([3.95, 0, 2.15], rotate([0, 90, 0], cylinder(h=0.06, r=0.66, fn=18))), [0.12, 0.12, 0.13])
# Chimney, flared at the top; the dome and safety valves.
m.add(translate([3.45, 0, 2.7], cylinder(h=1.1, r1=0.2, r2=0.24, fn=14)), black)
m.add(translate([3.45, 0, 3.75], cylinder(h=0.18, r1=0.24, r2=0.32, fn=14)), black)
m.add(translate([1.2, 0, 2.7], cylinder(h=0.4, r1=0.36, r2=0.3, fn=14)), brass)
m.add(translate([1.2, 0, 3.1], sphere(r=0.3, fn=12)), brass)
m.add(translate([-0.5, 0, 2.75], cylinder(h=0.35, r=0.12, fn=8)), brass)
# Side tanks.
for s in (-1, 1):
    m.add(box(-1.0, 2.7, 0.55, 1.22, 2.65, y=s * 1.05), green)
    m.add(box(-1.0, 2.7, 0.57, 2.45, 2.52, y=s * 1.05), brass)  # a lining band
# The cab: green, a window at the front either side of the boiler, side
# windows, a curved black roof.
m.add(box(-4.1, -1.4, 2.7, 1.22, 3.3), green)
for s in (-1, 1):
    m.add(box(-1.43, -1.38, 0.5, 2.55, 3.05, y=s * 0.95), glass)
    m.add(box(-3.4, -2.2, 0.04, 2.3, 3.0, y=s * 1.36), glass)
m.add(section(roof_profile(2.9, 3.3, 0.35), -4.25, -1.25), black)
# Bunker behind the cab, heaped with coal.
m.add(box(-4.2, -3.6, 2.4, 1.22, 2.6), green)
# Buffer beams, buffers, and a lamp on the smokebox.
buffers(m, 4.3, +1, beam=red)
buffers(m, -4.3, -1, beam=red)
m.add(translate([4.0, 0, 3.05], box(-0.12, 0.12, 0.25, 0, 0.3)), [0.95, 0.92, 0.7])
m.main()

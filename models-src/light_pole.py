"""Light pole — a night-skiing pole beside a lit run: a tapered steel
pole on a footing with a short arm at the top carrying two floodlights
aimed down and out over the run (+X, toward the run's middle).

The lamps' lenses are drawn by the renderer as glowing boxes after dark
(render/night_lights.go: poleLenses, at the lens faces here).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

pole_h = 10.0
m = Model()
m.add(translate([0, 0, -1.0], cylinder(h=1.35, r=0.35, fn=12)), "Silver")
m.add(cylinder(h=pole_h, r1=0.14, r2=0.08, fn=12), "SlateGray")
# The arm out over the run, and a lamp housing either side of its end.
m.add(translate([0.45, 0, pole_h - 0.15], cube([0.9, 0.12, 0.12], center=True)), "SlateGray")
for y in (-0.32, 0.32):
    m.add(translate([0.85, y, pole_h - 0.3], rotate([0, 35, 0], cube([0.36, 0.42, 0.22], center=True))), "DimGray")
m.main()

"""Jeep — a short, boxy off-roader: upright windshield, flat hood, chunky
tyres, a spare wheel on the tailgate. Seats four. Parts and conventions:
lib/car_kit.py.
"""

from lib.car_kit import hubcap, trim, tyre, vehicle, wheel_fn
from lib.mogul import *

len_ = 4.10

m = Model()
vehicle(m, len=len_, w=1.86, r=0.42, wb=2.45, sill=0.40,
        belt=1.12, hood=1.08, nose=0.04, tailin=0.02,
        roof=1.86, gb=(-1.95, 0.55), gr=(-1.95, 0.48))

# Spare wheel on the tailgate.
spare = (-len_ / 2 - 0.16, 0, 0.95)
m.add(translate(spare, rotate([0, 90, 0], cylinder(h=0.22, r=0.38, center=True, fn=wheel_fn))), tyre)
m.add(translate(spare, rotate([0, 90, 0], cylinder(h=0.24, r=0.18, center=True, fn=10))), hubcap)
# Grille slots across the nose.
m.add([translate([len_ / 2 - 0.02, i * 0.12 - 0.03, 0.70], cube([0.04, 0.06, 0.26])) for i in range(-3, 4)], trim)
m.main()

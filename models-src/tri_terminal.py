"""Tricable gondola terminal — a 3S's bottom / top station house
(lib/station_kit.add_cabin_house): the cabins off the haul rope, slow
round the bullwheel on a rail under each track rope, docking on a
platform up over the enclosed ground floor. Keep the footprint in step
with world.triStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_cabin_house

gap, spread = 3.2, 0.4  # world.TramGap, world.TramRopeSpread
floor = 2.5  # world.triPlatform
cabin_across = 3.85  # tri_cabin.py

m = Model()
add_cabin_house(m, gap, gap + cabin_across / 2 + 0.1, floor, 5.2, cabin_across, ropes=(-spread, spread))  # world.TriDockHeight
m.main()

"""Funitel terminal — a funitel's bottom / top station house
(lib/station_kit.add_cabin_house): the cabins off both haul ropes, slow
round the bullwheel on a rail under each rope, docking on a platform up
over the enclosed ground floor. Keep the footprint in step with
world.funitelStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_cabin_house

gap, spread = 3.6, 1.6  # world.FunitelGap, world.FunitelRopeSpread
floor = 2.2  # world.funitelPlatform
cabin_across = 3.1  # funitel_cabin.py

m = Model()
add_cabin_house(m, gap, gap + spread + 0.15, floor, 3.65, cabin_across, ropes=(-spread, spread))
m.main()

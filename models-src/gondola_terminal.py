"""Gondola terminal — the gondola's bottom / top station house
(lib/station_kit.add_cabin_house): the cabins come off the cable and run
slow round the bullwheel on a rail hung from a hall roof on six columns,
docking on a platform up over an enclosed ground floor.

The bullwheel is centred on the origin at cable height (BullwheelHeight
over the platform), its rim at the cable gap (CableGap). Riders get on
load_x in front of it on the riders' cable, where a red line marks the
platform; the line files in under it through the lobby in the back wall,
and riders getting off walk out the same way. The operator works inside.

Sides as lift_station.py: at the base riders go up on -Y; at the top the
terminal is turned round and riders arrive on +Y. Keep the footprint in
step with world.gondolaStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_cabin_house, cable_gap

floor = 2.2  # world.gondolaPlatform
cabin_w = 2.1  # gondola_cabin.py, square
sweep = cable_gap + 1.2  # the cabins' outer edge, with room

m = Model()
add_cabin_house(m, cable_gap, sweep, floor, 3.65, cabin_w)  # world.BullwheelHeight over the platform
m.main()

"""Lift terminal, wide — the 6-pack's detachable terminal: lift_terminal.py
built for its 3.6 m chairs (lib/station_kit.add_terminal). Keep the
footprint in step with world.detachWideStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_terminal

seat_w = 3.60  # the 6-pack's seat

m = Model()
add_terminal(m, seat_w)
m.main()

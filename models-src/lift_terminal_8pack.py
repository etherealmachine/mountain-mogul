"""Lift terminal, 8-pack — the 8-pack's detachable terminal: lift_terminal.py
built for its 4.8 m chairs on cables world.WideCableGap either side
(lib/station_kit.add_terminal). Keep the footprint in step with
world.detach8Station.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_terminal

seat_w = 4.80  # the 8-pack's seat
gap = 2.6  # world.WideCableGap

m = Model()
add_terminal(m, seat_w, gap)
m.main()

"""Lift terminal — a detachable lift's bottom / top terminal for quad-wide
chairs and cabins (the high-speed quad and the gondola); the 6-pack's is
lift_terminal_wide.py. Built by lib/station_kit.add_terminal: a long
barrel vault about as wide as the chairs on four posts, the bullwheel at
its back and the load point near its front.

The bullwheel is centred on the origin at the raised height, its rim at
the cable gap (CableGap). Riders get on load_x in front of it on the
riders' cable, where a red line marks the spot; the lift line runs back
from there along the riders' cable, under the roof, between the posts.

Sides as lift_station.py: at the base riders go up on -Y, the hut is on
+Y; at the top the terminal is turned round and riders arrive on +Y. Keep
the footprint in step with world.detachStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.station_kit import add_terminal

seat_w = 2.40  # the quad's seat; the gondola cabin is as wide

m = Model()
add_terminal(m, seat_w)
m.main()

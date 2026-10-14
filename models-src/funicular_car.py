"""Funicular car — a mountain funicular's car (rail_kit.py): four
stepped compartments on a sloping chassis, so their floors stay level on
a line of design_deg; red below, cream above, big windows, about 100
riders. Authored along the track (+X uphill), 11 m long; the renderer
pitches it with the line, so on a steeper or gentler one the
compartments lean a little (notes/next/Trains and Tunnels.md).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

design_deg = 30.0
red = [0.70, 0.12, 0.10]
cream = [0.92, 0.88, 0.76]
roof = [0.35, 0.36, 0.38]

m = Model()
# The chassis along the track, on two pairs of wheels.
m.add(box(-5.5, 5.5, 2.2, 0.55, 0.85), dark)
for x in (3.5, -3.5):
    wheelset(m, x, 0.45)
compartments = 4
step = 11.0 / compartments
for i in range(compartments):
    x = -5.5 + step * (i + 0.5)
    # A level box in the world: turned back by the design slope about
    # its floor's middle, standing on the chassis.
    room = union(
        translate([-step / 2 + 0.05, -1.3, 0], cube([step - 0.1, 2.6, 1.2])),  # red lower half
    )
    upper = translate([-step / 2 + 0.05, -1.3, 1.2], cube([step - 0.1, 2.6, 1.15]))
    top = translate([-step / 2, -1.35, 2.35], cube([step, 2.7, 0.15]))
    panes = [translate([-step / 2 + 0.25, s * 1.31 - 0.02, 1.3], cube([step - 0.5, 0.04, 0.85])) for s in (-1, 1)]
    def place(shape):
        return translate([x, 0, 0.85], rotate([0, design_deg, 0], shape))
    m.add(place(room), red)
    m.add(place(upper), cream)
    m.add(place(top), roof)
    m.add([place(p) for p in panes], glass)
m.main()

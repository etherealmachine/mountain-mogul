"""Funicular car — a mountain funicular's car (rail_kit.py): four
stepped compartments on a sloping chassis, so their floors stay level on
a line of design_deg; red below, cream above, big windows, about 100
riders. Authored along the track (+X uphill), 11 m long; the renderer
pitches it with the line, so on a steeper or gentler one the
compartments lean a little (notes/next/Trains and Tunnels.md).

Conventions, units, and axis docs: see models-src/README.md.
"""

import math

from lib.mogul import *
from lib.rail_kit import *

design_deg = 20.0
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
d = math.radians(design_deg)
c, s = math.cos(d), math.sin(d)
L = step / c  # a compartment's own length, so it spans step along the chassis
for k in range(compartments):
    ux = -5.5 + step * (k + 1)  # its uphill end

    # A level box in the world: turned back by the design slope about
    # its uphill floor corner, on the chassis; its downhill end rides up
    # off it, and a red skirt fills the gap.
    def place(shape, ux=ux):
        return translate([ux, 0, 0.85], rotate([0, design_deg, 0], shape))
    room = translate([-L + 0.05, -1.3, 0], cube([L - 0.1, 2.6, 1.2]))
    upper = translate([-L + 0.05, -1.3, 1.2], cube([L - 0.1, 2.6, 1.15]))
    top = translate([-L, -1.35, 2.35], cube([L, 2.7, 0.15]))
    panes = [translate([-L + 0.25, sd * 1.31 - 0.02, 1.3], cube([L - 0.5, 0.04, 0.85])) for sd in (-1, 1)]
    m.add(place(room), red)
    m.add(place(upper), cream)
    m.add(place(top), roof)
    m.add([place(p) for p in panes], glass)
    lo, hi = ux - step + 0.05, ux - 0.05
    m.add(hull_points([(x, y, z) for y in (-1.3, 1.3)
                       for (x, z) in ((lo, 0.85), (hi, 0.85), (lo, 0.85 + (ux - lo) * s / c))]), red)

# ── Rider slots ────────────────────────────────────────────────────────
# A hundred standing, twenty-five to a compartment on its level floor:
# five rows of five, 0.48 m along and 0.5 m across.
i = 0
for k in range(compartments):
    ux = -5.5 + step * (k + 1)
    for u in (-0.5, -0.98, -1.46, -1.94, -2.42):  # back from its uphill end, its own frame
        for v in (-1.0, -0.5, 0, 0.5, 1.0):
            z = 0.04
            m.slot(i, ux + u * c + z * s, v, 0.85 - u * s + z * c, "stand")
            i += 1
m.main()

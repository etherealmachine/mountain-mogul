"""Electric locomotive — a modern German-style electric (rail_kit.py),
after DB's class 146: a long traffic-red body with sloped cab fronts at
both ends, a light grey skirt and roof, two single-arm pantographs, two
three-metre bogies. 18.9 m over the buffers; the electric style's engine
(notes/next/Trains and Tunnels.md).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

red = [0.78, 0.08, 0.10]
light = [0.80, 0.81, 0.82]

m = Model()
for x in (5.6, -5.6):
    bogie(m, x, base=2.9, r=0.625)
m.add(box(-9.1, 9.1, 2.9, 1.15, 1.4), dark)  # underframe
# The body: full width low, inset at the ends higher up, so the cab
# fronts slope back.
lo = [(-9.0, -1.5), (9.0, -1.5), (9.0, 1.5), (-9.0, 1.5)]
m.add(hull(box(-9.0, 9.0, 3.0, 1.4, 2.5), box(-8.4, 8.4, 2.9, 3.6, 3.75)), red)
m.add(section(roof_profile(2.9, 3.75, 0.3), -8.4, 8.4), light)
stripe(m, -8.9, 8.9, 3.0, 1.4, 1.7, light)
# Cab windscreens, sloped with the front, and side windows.
for e in (-1, 1):
    m.add(hull(box(e * 8.96 - 0.02, e * 8.96 + 0.02, 2.5, 2.65, 2.7), box(e * 8.51 - 0.02, e * 8.51 + 0.02, 2.4, 3.45, 3.5)), glass)
    m.add(box(e * 7.6 - 0.5, e * 7.6 + 0.5, 3.02, 2.6, 3.3), glass)
    for s in (-1, 1):  # lights
        m.add(translate([e * 9.03, s * 0.95, 1.9], box(-0.05, 0.05, 0.3, 0, 0.18)), [1.0, 0.97, 0.85])
    buffers(m, e * 9.1, e, beam=dark, beam_w=2.9)
# Grilles along the sides, and the roof gear and pantographs.
window_band(m, -6.0, 6.0, 3.0, 2.7, 3.2, gap=0.35, pane=0.9, color=[0.55, 0.56, 0.58])
m.add(box(-3.0, 3.0, 1.4, 4.0, 4.3), [0.45, 0.46, 0.48])
pantograph(m, 5.2, 4.05)
pantograph(m, -5.2, 4.05)
m.main()

"""High-speed power car — a streamlined high-speed trainset's end car
(rail_kit.py), after the ICE and TGV: a white body with a long tapering
nose, a wraparound windscreen, a red stripe, a pantograph at the back.
20.5 m long, its nose to +X; the train has one at each end, the rear one
turned round (notes/next/Trains and Tunnels.md).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

white = [0.93, 0.94, 0.95]
red = [0.80, 0.06, 0.10]

m = Model()
for x in (3.8, -6.6):
    bogie(m, x, base=3.0, r=0.46)
m.add(box(-10.0, 7.5, 2.8, 1.05, 1.25), dark)
# The body's cross-section: a rounded box, 2.95 wide, 1.25 to 4.0 up.
def ring(x, w, z0, z1, r):
    hw = w / 2
    pts = []
    for cy, cz, a0 in ((hw - r, z1 - r, 0), (-hw + r, z1 - r, 90), (-hw + r, z0 + r, 180), (hw - r, z0 + r, 270)):
        for k in range(4):
            a = math.radians(a0 + 90 * k / 3)
            pts.append((x, cy + r * math.cos(a), cz + r * math.sin(a)))
    return pts
# The tail half full section; the nose a run of shrinking, dropping
# sections out to the tip.
stations = [(-10.0, 2.95, 1.25, 4.0, 0.55), (2.0, 2.95, 1.25, 4.0, 0.55), (5.0, 2.9, 1.25, 3.85, 0.6),
            (7.5, 2.6, 1.25, 3.3, 0.6), (9.0, 2.0, 1.3, 2.6, 0.5), (10.0, 1.2, 1.35, 1.95, 0.3), (10.4, 0.5, 1.45, 1.65, 0.12)]
for a, b in zip(stations, stations[1:]):
    m.add(hull_points(ring(*a) + ring(*b)), white)
# The windscreen: a dark wrap over the nose's upper slope.
m.add(hull_points(ring(7.3, 2.66, 3.0, 3.36, 0.5) + ring(8.9, 2.06, 2.45, 2.66, 0.45)), glass)
# Red stripe and window band on the body.
stripe(m, -9.8, 7.2, 2.95, 1.55, 1.75, red)
window_band(m, -9.0, 1.0, 2.95, 2.55, 3.1, color=glass)
m.add(box(-9.6, -6.8, 1.6, 4.0, 4.15), [0.6, 0.62, 0.64])
pantograph(m, -8.0, 4.15)
m.main()

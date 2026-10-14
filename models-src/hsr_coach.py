"""High-speed coach — a streamlined trainset's middle car (rail_kit.py):
white, a continuous window band, a red stripe, smooth rounded roof, two
bogies. 24.8 m long, close-coupled (no buffers).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

white = [0.93, 0.94, 0.95]
red = [0.80, 0.06, 0.10]

m = Model()
for x in (8.8, -8.8):
    bogie(m, x, base=2.5, r=0.46)
prof = [(1.475, 1.25), (1.475, 3.45), (1.3, 3.85), (0.9, 4.0), (-0.9, 4.0), (-1.3, 3.85), (-1.475, 3.45), (-1.475, 1.25)]
m.add(section(prof, -12.4, 12.4), white)
m.add(box(-12.3, 12.3, 2.8, 1.05, 1.25), dark)
window_band(m, -11.4, 11.4, 2.95, 2.5, 3.15)
stripe(m, -12.3, 12.3, 2.95, 1.55, 1.75, red)
for e in (-1, 1):
    m.add(box(e * 11.5 - 0.5, e * 11.5 + 0.5, 2.99, 1.35, 3.35), [0.7, 0.72, 0.74])
    m.add(box(e * 12.4 - 0.05, e * 12.4 + 0.05, 2.6, 1.4, 3.7), [0.15, 0.15, 0.16])  # gangway bellows
m.main()

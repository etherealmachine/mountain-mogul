"""Chairlift chair — a single chair, one rider, after Mad River Glen's
(lib/chair_kit.py).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import chair, foot_top_z
from lib.mogul import Model

m = Model()
chair(m, 0.72, [0.62, 0.20, 0.16], hanger=0.08)
m.slot(0, 0, 0, foot_top_z)  # the one rider, centred
m.main()

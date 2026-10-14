"""Chairlift chair — a high-speed detachable 6-pack, built from the chair
kit (lib/chair_kit.py) with a heavier hanger for the bigger frame. Six
riders, 0.6 m apart. chair_6pack_bubble*.py add the bubble cover.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bench
from lib.mogul import Model

m = Model()
bench(m, 3.60, [0.16, 0.20, 0.30], hanger=0.14)
m.main()

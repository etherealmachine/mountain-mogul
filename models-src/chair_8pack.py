"""Chairlift chair — a high-speed detachable 8-pack, the biggest chair
(lib/chair_kit.py): eight riders 0.6 m apart on a 4.8 m bench, red
cushions, the heaviest hanger. chair_8pack_bubble*.py add the bubble
cover.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bench
from lib.mogul import Model

m = Model()
bench(m, 4.80, [0.62, 0.20, 0.16], hanger=0.16)
m.main()

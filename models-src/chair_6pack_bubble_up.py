"""Chairlift chair — the 6-pack (chair_6pack.py) with its bubble cover's
frame up, as in the terminals (lib/chair_kit.bubble); the tinted hood is
chair_6pack_hood_up.py, drawn see-through over it.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bench, bubble
from lib.mogul import Model

m = Model()
bench(m, 3.60, [0.16, 0.20, 0.30], hanger=0.14)
bubble(m, 3.60, down=False, hood=False)
m.main()

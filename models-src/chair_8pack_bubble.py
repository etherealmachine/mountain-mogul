"""Chairlift chair — the 8-pack (chair_8pack.py) with its bubble cover's
frame down over the riders, as on the line (lib/chair_kit.bubble); the tinted hood is
chair_8pack_hood.py, drawn see-through over it.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bench, bubble
from lib.mogul import Model

m = Model()
bench(m, 4.80, [0.62, 0.20, 0.16], hanger=0.16)
bubble(m, 4.80, down=True, hood=False)
m.main()

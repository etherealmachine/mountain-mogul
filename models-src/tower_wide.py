"""Lift tower, wide — the 8-pack's tower (lib/tower_kit.py): its cables
world.WideCableGap either side of the pole, so its 4.8 m chairs pass each
other clear.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.tower_kit import chair_tower

m = Model()
chair_tower(m, (2.6, -2.6))  # world.WideCableGap
m.main()

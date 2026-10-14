"""Lift tower — chairlift tower (lib/tower_kit.py), its cables CableGap
either side of the pole at world.TowerHeight. Chairs hang from the cable
down to about 1.9 m below it and a quad's reaches to 0.3 m off the
centreline.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.tower_kit import chair_tower

m = Model()
chair_tower(m, (1.5, -1.5))  # world.CableGap
m.main()

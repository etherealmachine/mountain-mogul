"""Funitel tower — a funitel's tower (lib/tower_kit.py): each way's two
haul ropes, world.FunitelRopeSpread either side of its track
world.FunitelGap off the pole, at world.FunitelLineHeight, a sheave train
under each.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import Model
from lib.tower_kit import chair_tower

gap, spread = 3.6, 1.6  # world.FunitelGap, world.FunitelRopeSpread

m = Model()
chair_tower(m, (gap + spread, gap - spread, -gap + spread, -gap - spread), cable_h=22.0)  # world.FunitelLineHeight
m.main()

"""Chairlift chair — a fixed-grip double, built from the chair kit
(lib/chair_kit.py: grip, hanger, frame, cushions, raised restraint bar,
footrest). Hangs below the cable at its attachment point.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import chair, foot_top_z
from lib.mogul import Model

seat_w = 1.50  # 2-person, mid of real range (1.4 m for 2-seater)

m = Model()
chair(m, seat_w, [0.70, 0.22, 0.20], hanger=0.10)

# ── Passenger seat anchors ─────────────────────────────────────────────
# Slot positions for the simulation to anchor riders to, written as
# `# slot` lines (game coords). The agent mesh's origin is at its feet, so
# slot_z is the top of the footrest: the rider resting their boots on it.
slot_x = 0  # centred along seat depth
slot_lateral_y = 0.40  # half the space between two riders
slot_z = foot_top_z

m.slot(0, slot_x, slot_lateral_y, slot_z)
m.slot(1, slot_x, -slot_lateral_y, slot_z)
m.main()

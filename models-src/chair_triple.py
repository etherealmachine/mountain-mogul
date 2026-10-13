"""Chairlift chair — a fixed-grip triple, built from the chair kit
(lib/chair_kit.py).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import chair, foot_top_z
from lib.mogul import Model

seat_w = 1.90  # 3-person, real-world triples run 1.8–2.0 m wide

m = Model()
chair(m, seat_w, [0.20, 0.52, 0.32], hanger=0.11)

# ── Passenger seat anchors ─────────────────────────────────────────────
# Three slots across the seat, 0.60 m centre-to-centre like the quad,
# leaving 0.35 m to each outer edge; written as `# slot` lines (game
# coords).
slot_x = 0  # centred along seat depth
slot_outer_y = 0.60
slot_z = foot_top_z  # top of the footrest

m.slot(0, slot_x, -slot_outer_y, slot_z)
m.slot(1, slot_x, 0, slot_z)
m.slot(2, slot_x, slot_outer_y, slot_z)
m.main()

"""Chairlift chair — a high-speed detachable 6-pack, built from the chair
kit (lib/chair_kit.py) with a heavier hanger for the bigger frame.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import chair, foot_top_z
from lib.mogul import Model

seat_w = 3.60  # 6-person: 6 × 0.60 m centre-to-centre

m = Model()
chair(m, seat_w, [0.16, 0.20, 0.30], hanger=0.14)

# ── Passenger seat anchors ─────────────────────────────────────────────
# Six slots at y = ±0.30, ±0.90, ±1.50 (0.60 m centre-to-centre).
slot_x = 0
slot_y1 = 0.30  # inner pair
slot_y2 = 0.90  # middle pair
slot_y3 = 1.50  # outer pair
slot_z = foot_top_z

m.slot(0, slot_x, -slot_y3, slot_z)
m.slot(1, slot_x, -slot_y2, slot_z)
m.slot(2, slot_x, -slot_y1, slot_z)
m.slot(3, slot_x, slot_y1, slot_z)
m.slot(4, slot_x, slot_y2, slot_z)
m.slot(5, slot_x, slot_y3, slot_z)
m.main()

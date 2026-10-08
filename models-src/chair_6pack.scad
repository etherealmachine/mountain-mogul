// Chairlift chair — a high-speed detachable 6-pack, built from the chair
// kit (lib/chair_kit.scad) with a heavier hanger for the bigger frame.
//
// Conventions, units, and axis docs: see models-src/README.md.

include <lib/chair_kit.scad>

seat_w = 3.60; // 6-person: 6 × 0.60 m centre-to-centre

chair(seat_w, [0.16, 0.20, 0.30], hanger = 0.14);

// ── Passenger seat anchors ─────────────────────────────────────────────
// Six slots at y = ±0.30, ±0.90, ±1.50 (0.60 m centre-to-centre).
slot_x       = 0;
slot_y1      = 0.30;   // inner pair
slot_y2      = 0.90;   // middle pair
slot_y3      = 1.50;   // outer pair
slot_z       = foot_top_z;

echo("MOGUL_META", "slot", 0, slot_x, -slot_y3, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, -slot_y2, slot_z);
echo("MOGUL_META", "slot", 2, slot_x, -slot_y1, slot_z);
echo("MOGUL_META", "slot", 3, slot_x,  slot_y1, slot_z);
echo("MOGUL_META", "slot", 4, slot_x,  slot_y2, slot_z);
echo("MOGUL_META", "slot", 5, slot_x,  slot_y3, slot_z);

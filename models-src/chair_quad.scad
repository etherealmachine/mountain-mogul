// Chairlift chair — a quad (fixed-grip and high-speed quads share it),
// built from the chair kit (lib/chair_kit.scad).
//
// Conventions, units, and axis docs: see models-src/README.md.

include <lib/chair_kit.scad>

seat_w = 2.40; // 4-person, real-world quads run 2.3–2.5 m wide

chair(seat_w, [0.22, 0.42, 0.68], hanger = 0.12);

// ── Passenger seat anchors ─────────────────────────────────────────────
// Four slots laterally arranged at y = ±0.30 and ±0.90 metres — 0.60 m
// centre-to-centre between adjacent riders, 0.30 m clearance to the
// outer edges of the 2.40 m seat. scad2obj captures these as `# slot`
// comment lines in chair_quad.obj (already converted to game coords).
slot_x        = 0;           // centred along seat depth
slot_inner_y  = 0.30;        // inner pair
slot_outer_y  = 0.90;        // outer pair
slot_z        = foot_top_z;  // top of the footrest

echo("MOGUL_META", "slot", 0, slot_x, -slot_outer_y, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, -slot_inner_y, slot_z);
echo("MOGUL_META", "slot", 2, slot_x,  slot_inner_y, slot_z);
echo("MOGUL_META", "slot", 3, slot_x,  slot_outer_y, slot_z);

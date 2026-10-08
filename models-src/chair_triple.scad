// Chairlift chair — a fixed-grip triple, built from the chair kit
// (lib/chair_kit.scad).
//
// Conventions, units, and axis docs: see models-src/README.md.

include <lib/chair_kit.scad>

seat_w = 1.90; // 3-person, real-world triples run 1.8–2.0 m wide

chair(seat_w, [0.20, 0.52, 0.32], hanger = 0.11);

// ── Passenger seat anchors ─────────────────────────────────────────────
// Three slots across the seat, 0.60 m centre-to-centre like the quad,
// leaving 0.35 m to each outer edge. scad2obj captures these as `# slot`
// comment lines in chair_triple.obj (already converted to game coords).
slot_x       = 0;           // centred along seat depth
slot_outer_y = 0.60;
slot_z       = foot_top_z;  // top of the footrest

echo("MOGUL_META", "slot", 0, slot_x, -slot_outer_y, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, 0, slot_z);
echo("MOGUL_META", "slot", 2, slot_x,  slot_outer_y, slot_z);

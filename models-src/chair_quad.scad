// Fixed-grip quad chairlift chair — a 4-seater. Same anatomy as the
// double in chair.scad (suspension bar, seat, backrest, foot bar) just
// wider, with four passenger slots instead of two.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = forward (direction the chair travels along the cable)
//     +Y = seat width (lateral, perpendicular to cable)
//     +Z = up
//
//   Origin sits at the cable-attachment point, *not* on the ground; the
//   chair hangs entirely in -Z. The renderer places the origin at the
//   computed cable position, so don't shift the model up to Z=0.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
suspension_thick = 0.12; // square cross-section — slightly fatter than the double's 0.10
suspension_len   = 1.30; // from cable down to top of seat

seat_w     = 2.40; // 4-person, real-world fixed quads run 2.3–2.5 m wide
seat_depth = 0.70; // front-to-back, along X — matched to the double
seat_thick = 0.25; // vertical

back_thick  = 0.15; // depth (along X) of the backrest panel
back_height = 0.50; // vertical

foot_w     = 2.40; // matches seat width (Y)
foot_depth = 0.10; // along X
foot_thick = 0.05;
foot_drop  = 0.30; // distance below seat bottom to the foot bar

// Derived Z coordinates (everything hangs in -Z from the cable origin).
seat_top_z  = -suspension_len;             // -1.30
seat_bot_z  = seat_top_z - seat_thick;     // -1.55
foot_bot_z  = seat_bot_z - foot_drop;      // -1.85

// ── Parts ──────────────────────────────────────────────────────────────
// All parts wrapped in color("white") so the chair batch's per-instance
// tint (grey when empty, blue when carrying passengers) reads cleanly.
module suspension() {
    color("white")
        translate([0, 0, seat_top_z / 2])
            cube([suspension_thick, suspension_thick, suspension_len],
                 center = true);
}

module seat() {
    color("white")
        translate([0, 0, (seat_top_z + seat_bot_z) / 2])
            cube([seat_depth, seat_w, seat_thick], center = true);
}

module backrest() {
    back_x = -seat_depth / 2 + back_thick / 2;
    back_z = seat_top_z + back_height / 2;
    color("white")
        translate([back_x, 0, back_z])
            cube([back_thick, seat_w, back_height], center = true);
}

module footbar() {
    bar_z = foot_bot_z + foot_thick / 2;
    color("white")
        translate([0, 0, bar_z])
            cube([foot_depth, foot_w, foot_thick], center = true);
}

module chair() {
    suspension();
    seat();
    backrest();
    footbar();
}

chair();

// ── Passenger seat anchors ─────────────────────────────────────────────
// Four slots laterally arranged at y = ±0.30 and ±0.90 metres — 0.60 m
// centre-to-centre between adjacent riders, 0.30 m clearance to the
// outer edges of the 2.40 m seat. scad2obj captures these as `# slot`
// comment lines in chair_quad.obj (already converted to game coords).
slot_x        = 0;                            // centred along seat depth
slot_inner_y  = 0.30;                         // inner pair
slot_outer_y  = 0.90;                         // outer pair
slot_z        = foot_bot_z + foot_thick;      // top of the foot bar

echo("MOGUL_META", "slot", 0, slot_x, -slot_outer_y, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, -slot_inner_y, slot_z);
echo("MOGUL_META", "slot", 2, slot_x,  slot_inner_y, slot_z);
echo("MOGUL_META", "slot", 3, slot_x,  slot_outer_y, slot_z);

// High-speed detachable 6-pack chairlift chair — a 6-seater. Same anatomy
// as chair_quad.scad (suspension bar, seat, backrest, foot bar) but wider,
// with six passenger slots and a slightly heavier suspension to suggest the
// bigger frame of a detachable grip.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = forward (direction the chair travels along the cable)
//     +Y = seat width (lateral, perpendicular to cable)
//     +Z = up
//
//   Origin sits at the cable-attachment point, *not* on the ground; the
//   chair hangs entirely in -Z.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
suspension_thick = 0.14; // slightly heavier than the quad (0.12) for the bigger frame
suspension_len   = 1.30; // from cable down to top of seat

seat_w     = 3.60; // 6-person: 6 × 0.60 m centre-to-centre
seat_depth = 0.70;
seat_thick = 0.25;

back_thick  = 0.15;
back_height = 0.50;

foot_w     = 3.60;
foot_depth = 0.10;
foot_thick = 0.05;
foot_drop  = 0.30;

// Derived Z coordinates.
seat_top_z  = -suspension_len;
seat_bot_z  = seat_top_z - seat_thick;
foot_bot_z  = seat_bot_z - foot_drop;

// ── Parts ──────────────────────────────────────────────────────────────
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
// Six slots at y = ±0.30, ±0.90, ±1.50 (0.60 m centre-to-centre).
slot_x       = 0;
slot_y1      = 0.30;   // inner pair
slot_y2      = 0.90;   // middle pair
slot_y3      = 1.50;   // outer pair
slot_z       = foot_bot_z + foot_thick;

echo("MOGUL_META", "slot", 0, slot_x, -slot_y3, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, -slot_y2, slot_z);
echo("MOGUL_META", "slot", 2, slot_x, -slot_y1, slot_z);
echo("MOGUL_META", "slot", 3, slot_x,  slot_y1, slot_z);
echo("MOGUL_META", "slot", 4, slot_x,  slot_y2, slot_z);
echo("MOGUL_META", "slot", 5, slot_x,  slot_y3, slot_z);

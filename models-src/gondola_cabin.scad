// Monocable detachable (MDG) gondola cabin — 8 passengers in two rows of four.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = forward (direction of travel along the cable)
//     +Y = lateral (perpendicular to cable)
//     +Z = up (SCAD Z-up; scad2obj rotates to game Y-up at export)
//
//   Origin sits at the cable-grip point. The entire cabin hangs in -Z.
//   The hanger arm runs from the grip down to the cabin roof; the body
//   hangs below that. Keep colours near white so the per-instance tint
//   (grey = empty, blue = occupied) reads cleanly.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
hanger_thick = 0.14;  // square cross-section of the grip arm
hanger_len   = 0.80;  // grip to cabin roof

cabin_w = 2.40;  // lateral (Y) — fits 4 riders side-by-side at 0.60 m spacing
cabin_d = 1.30;  // along-cable (X) — two facing rows with ~0.30 m knee clearance
cabin_h = 2.10;  // floor to ceiling
wall_t  = 0.08;  // wall thickness

// Divide the side-wall height into three bands.
lower_frac = 0.28;  // solid skirt below windows
win_frac   = 0.47;  // window strip
upper_frac = 0.25;  // solid header above windows (= 1 - lower - win)

lower_h = cabin_h * lower_frac;   // 0.588
win_h   = cabin_h * win_frac;     // 0.987
upper_h = cabin_h * upper_frac;   // 0.525

// Z anchors (everything hangs in -Z).
cabin_top_z = -hanger_len;                           // -0.80
cabin_bot_z = cabin_top_z - cabin_h;                 // -2.90

lower_bot_z = cabin_bot_z;                           // -2.90
lower_top_z = lower_bot_z + lower_h;                 // -2.312
win_bot_z   = lower_top_z;                           // -2.312
win_top_z   = win_bot_z   + win_h;                   // -1.325
upper_bot_z = win_top_z;                             // -1.325  (check: + 0.525 = -0.80 ✓)

lower_ctr_z = (lower_bot_z + lower_top_z) / 2;      // -2.606
win_ctr_z   = (win_bot_z   + win_top_z)   / 2;      // -1.8185
upper_ctr_z = (upper_bot_z + cabin_top_z) / 2;      // -1.0625
cabin_ctr_z = (cabin_top_z + cabin_bot_z) / 2;      // -1.85

// ── Parts ──────────────────────────────────────────────────────────────
module hanger() {
    color("white")
        translate([0, 0, -hanger_len / 2])
            cube([hanger_thick, hanger_thick, hanger_len], center = true);
}

module cabin() {
    // Roof and floor panels (full footprint).
    color("white") {
        translate([0, 0, cabin_top_z - wall_t / 2])
            cube([cabin_d, cabin_w, wall_t], center = true);
        translate([0, 0, cabin_bot_z + wall_t / 2])
            cube([cabin_d, cabin_w, wall_t], center = true);
    }

    // Front and rear walls (±X faces, full height, full width).
    color("white") {
        translate([ cabin_d / 2 - wall_t / 2, 0, cabin_ctr_z])
            cube([wall_t, cabin_w, cabin_h], center = true);
        translate([-cabin_d / 2 + wall_t / 2, 0, cabin_ctr_z])
            cube([wall_t, cabin_w, cabin_h], center = true);
    }

    // Long side walls (±Y faces, parallel to cable direction).
    // Three bands: solid skirt, window strip, solid header.
    for (side = [-1, 1]) {
        y = side * (cabin_w / 2 - wall_t / 2);
        inner_w = cabin_d - 2 * wall_t;  // wall span between front/rear walls

        // Lower solid skirt.
        color("white")
            translate([0, y, lower_ctr_z])
                cube([inner_w, wall_t, lower_h], center = true);

        // Window glass strip.
        color([0.70, 0.88, 1.00])
            translate([0, y, win_ctr_z])
                cube([inner_w, wall_t, win_h], center = true);

        // Upper solid header.
        color("white")
            translate([0, y, upper_ctr_z])
                cube([inner_w, wall_t, upper_h], center = true);
    }
}

module gondola() {
    hanger();
    cabin();
}

gondola();

// ── Passenger seat anchors ─────────────────────────────────────────────
// Eight riders in two rows of four (front row at +X, rear at -X).
// Riders stand; slot_z is the cabin floor level (just above the floor panel).
row_x_front =  0.35;
row_x_rear  = -0.35;
slot_y_outer = 0.90;
slot_y_inner = 0.30;
slot_z       = cabin_bot_z + wall_t;   // -2.82 — standing on the floor

// Front row (slots 0–3, left to right in travel direction).
echo("MOGUL_META", "slot", 0, row_x_front, -slot_y_outer, slot_z);
echo("MOGUL_META", "slot", 1, row_x_front, -slot_y_inner, slot_z);
echo("MOGUL_META", "slot", 2, row_x_front,  slot_y_inner, slot_z);
echo("MOGUL_META", "slot", 3, row_x_front,  slot_y_outer, slot_z);
// Rear row (slots 4–7).
echo("MOGUL_META", "slot", 4, row_x_rear, -slot_y_outer, slot_z);
echo("MOGUL_META", "slot", 5, row_x_rear, -slot_y_inner, slot_z);
echo("MOGUL_META", "slot", 6, row_x_rear,  slot_y_inner, slot_z);
echo("MOGUL_META", "slot", 7, row_x_rear,  slot_y_outer, slot_z);

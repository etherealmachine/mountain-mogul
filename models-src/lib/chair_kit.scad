// Chair kit — the chairlift chair shared by the double, quad, and 6-pack
// (chair.scad, chair_quad.scad, chair_6pack.scad), which differ in width,
// cushion colour, and hanger weight.
//
//     +X = forward (direction the chair travels along the cable)
//     +Y = seat width (lateral, perpendicular to cable)
//     +Z = up
//
// The origin is the cable-attachment point and the chair hangs in -Z. A
// grip clamps the cable; a hanger drops behind the riders to a yoke over
// the backrest; side frames carry the seat, armrests, and a restraint bar
// (shown raised); a footrest hangs under the riders' feet. Riders are
// placed by the slots with their feet on the footrest, which hangs
// cable_h below the cable: the cable's height at a station
// (world.BullwheelHeight), so at the bullwheel a rider's skis are on the
// snow as the chair picks them up or sets them down. Everything else
// hangs from the seat height that gives.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 10;

// ── Shared dimensions (metres) ─────────────────────────────────────────
cable_h       = 3.65;  // world.BullwheelHeight
chair_depth   = 0.66;  // seat front to back (X)
foot_top_z    = -cable_h;          // footrest top: where riders' feet go (the slots)
seat_top_z    = foot_top_z + 0.50; // top of the seat cushion
seat_cushion  = 0.12;
seat_frame_z  = seat_top_z - seat_cushion - 0.04; // bar under the cushion
back_x        = -chair_depth / 2;                 // back edge of the seat
back_bot_z    = seat_top_z + 0.02;
back_top_z    = seat_top_z + 0.38;
back_cushion  = 0.08;
yoke_z        = seat_top_z + 0.44; // the bar over the backrest
restraint_z   = seat_top_z + 0.78; // the raised restraint bar, clear of riders' heads
grip_len      = 0.36;
foot_x        = 0;     // under the riders, at the slots' X
bar          = 0.06;   // frame tube section
frame_color   = [0.24, 0.26, 0.29];
grip_color    = [0.55, 0.57, 0.60];

// A box between two points, `s` thick: a frame tube.
module tube(a, b, s = bar) {
    hull() {
        translate(a) cube(s, center = true);
        translate(b) cube(s, center = true);
    }
}

// ── The chair ──────────────────────────────────────────────────────────
// w: seat width; cushion: seat and backrest colour; hanger: hanger section.
module chair(w, cushion, hanger = 0.10) {
    half = w / 2;
    ex = half + 0.03; // side frames, just outside the seat

    // Grip on the cable.
    color(grip_color)
        translate([0, 0, -0.02])
            cube([grip_len, 0.16, 0.20], center = true);

    color(frame_color) {
        // Hanger: down from the grip, then back over the riders' heads to
        // the yoke.
        tube([0, 0, -0.08], [0, 0, yoke_z + 0.18], hanger);
        tube([0, 0, yoke_z + 0.18], [back_x - 0.03, 0, yoke_z], hanger);
        // Yoke over the backrest, the full width.
        tube([back_x - 0.03, -ex, yoke_z], [back_x - 0.03, ex, yoke_z]);
        for (s = [-1, 1]) {
            y = s * ex;
            // Side frame: down the back, then forward under the seat.
            tube([back_x - 0.03, y, yoke_z], [back_x - 0.03, y, seat_frame_z]);
            tube([back_x - 0.03, y, seat_frame_z], [-back_x, y, seat_frame_z]);
            // Armrest with a post at its front.
            tube([back_x - 0.03, y, seat_top_z + 0.24], [0.14, y, seat_top_z + 0.24]);
            tube([0.14, y, seat_top_z + 0.24], [0.14, y, seat_frame_z]);
            // Restraint bar arm, raised: from the yoke up and forward.
            tube([back_x - 0.03, y, yoke_z], [0.30, y, restraint_z], 0.05);
            // Footrest hanger from the front of the seat.
            tube([-back_x - 0.02, s * (half - 0.08), seat_frame_z],
                 [foot_x + 0.05, s * (half - 0.08), foot_top_z - 0.02], 0.04);
        }
        // Restraint bar across the front, raised clear of the riders.
        tube([0.30, -ex, restraint_z], [0.30, ex, restraint_z], 0.05);
        // Cross tube under the front of the seat.
        tube([-back_x, -ex, seat_frame_z], [-back_x, ex, seat_frame_z]);
        // Footrest: the riders stand their skis on it.
        translate([foot_x, 0, foot_top_z - 0.025])
            cube([0.12, w - 0.1, 0.05], center = true);
    }

    color(cushion) {
        // Seat cushion.
        translate([0, 0, seat_top_z - seat_cushion / 2])
            cube([chair_depth, w, seat_cushion], center = true);
        // Backrest cushion, leaning back a touch.
        translate([back_x + back_cushion / 2, 0, back_bot_z])
            rotate([0, -8, 0])
                translate([0, 0, (back_top_z - back_bot_z) / 2])
                    cube([back_cushion, w, back_top_z - back_bot_z], center = true);
    }
}

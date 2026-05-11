// Chairlift chair — a simple two-seater with suspension bar, seat,
// backrest, and foot bar. Hangs below the cable at its attachment point.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = forward (direction the chair travels along the cable)
//     +Y = seat width (lateral, perpendicular to cable)
//     +Z = up
//
//   The rider sits facing +X with the backrest at -X. The dynamic-batch
//   shader rotates each instance around world Y so +X aligns with motion,
//   so chairs always look like they're heading down the cable.
//
//   Origin sits at the cable-attachment point, *not* on the ground; the
//   chair hangs entirely in -Z. The renderer places the origin at the
//   computed cable position, so don't shift the model up to Z=0.
//
// Per-vertex colours are kept near white because the chair batch supplies
// a per-instance tint (grey when empty, blue when carrying passengers).
// Anything coloured darker than white will mute that signal.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
suspension_thick = 0.10; // square cross-section
suspension_len   = 1.30; // from cable down to top of seat

seat_w     = 1.50; // 2-person, mid of real range (1.4 m for 2-seater) — lateral, along Y
seat_depth = 0.70; // front-to-back, along X
seat_thick = 0.25; // vertical

back_thick  = 0.15; // depth (along X) of the backrest panel
back_height = 0.50; // vertical

foot_w     = 1.50; // matches seat width (Y)
foot_depth = 0.10; // along X
foot_thick = 0.05;
foot_drop  = 0.30; // distance below seat bottom to the foot bar

// Derived Z coordinates (everything hangs in -Z from the cable origin).
seat_top_z  = -suspension_len;             // -1.30
seat_bot_z  = seat_top_z - seat_thick;     // -1.55
foot_bot_z  = seat_bot_z - foot_drop;      // -1.85, top of foot bar at this Z

// ── Parts ──────────────────────────────────────────────────────────────
// Everything is wrapped in explicit color() blocks because the chair batch
// supplies a per-instance tint (grey when empty, blue when carrying
// passengers) that is multiplied with the per-vertex colour — anything
// darker than white would mute that signal. Without explicit color(),
// OpenSCAD's 3MF export labels primitives with its preview yellow, which
// the scad2obj converter has no way to distinguish from a deliberate
// authored colour.
module suspension() {
    // Square bar from cable (z=0) down to the top of the seat.
    color("white")
        translate([0, 0, seat_top_z / 2])
            cube([suspension_thick, suspension_thick, suspension_len],
                 center = true);
}

module seat() {
    // Slab below the suspension bar. Centred on origin in X and Y.
    color("white")
        translate([0, 0, (seat_top_z + seat_bot_z) / 2])
            cube([seat_depth, seat_w, seat_thick], center = true);
}

module backrest() {
    // Thin panel rising from the back edge of the seat (-X side, behind
    // the rider). Slightly inset from the very edge so it sits on the
    // seat rather than overhanging.
    back_x = -seat_depth / 2 + back_thick / 2;
    back_z = seat_top_z + back_height / 2;
    color("white")
        translate([back_x, 0, back_z])
            cube([back_thick, seat_w, back_height], center = true);
}

module footbar() {
    // Slim bar parallel to the seat, hanging below it for riders' feet.
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
// Slot positions (SCAD coords) for the simulation to anchor riders to.
// scad2obj captures these from openscad's echo() output and bakes them
// into chair.obj as `# slot` comment lines (already converted to game
// coords). See tools/scad2obj/main.go for the protocol.
//
// Conventions: +X = forward, +Y = lateral, +Z = up. The agent mesh's
// origin is at its feet, so slot_z aligns with the chair feature where
// we want feet to land — the top of the foot bar reads as the rider
// resting their boots on it.
slot_x         = 0;                              // centred along seat depth
slot_lateral_y = 0.40;                           // half the space between two riders
slot_z         = foot_bot_z + foot_thick;        // top of the foot bar
echo("MOGUL_META", "slot", 0, slot_x,  slot_lateral_y, slot_z);
echo("MOGUL_META", "slot", 1, slot_x, -slot_lateral_y, slot_z);

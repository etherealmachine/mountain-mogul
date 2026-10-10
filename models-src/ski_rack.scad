// Ski rack — a wooden A-frame rack for ten pairs of skis, set on the snow
// by a lodge door.
//
// A top rail at shoulder height on an A-frame at each end, and a slotted
// foot rail low down in front (−Y) where the tails stand; skis lean back
// against the top rail. The game stands each pair in front of the rail
// (world.Building.SkiRackSlot: 0.3 m apart along X, 0.3 m out on the
// front side).
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 8;

len_x    = 3.2;   // rail length
rail_h   = 1.15;  // top rail height
rail_w   = 0.09;  // rail cross-section
leg_w    = 0.08;
leg_foot = 0.45;  // how far each A-frame leg spreads from the centre line
foot_h   = 0.22;  // foot rail height
foot_y   = 0.30;  // foot rail distance in front of the top rail
slot_n   = 10;
slot_pitch = 0.3;

wood  = [0.55, 0.38, 0.22];
dark  = [0.32, 0.22, 0.14];

echo("MOGUL_META", "footprint", len_x / 2 + 0.1, 0.55);

// One leg of an A-frame from the ground at (x, y) up to the rail.
module leg(x, y) {
    hull() {
        translate([x, y, 0]) cube([leg_w, leg_w, 0.01], center = true);
        translate([x, 0, rail_h]) cube([leg_w, leg_w, 0.01], center = true);
    }
}

color(wood) {
    // Top rail.
    translate([0, 0, rail_h]) cube([len_x, rail_w, rail_w], center = true);
    // A-frames at both ends.
    for (x = [-len_x / 2 + 0.1, len_x / 2 - 0.1]) {
        leg(x, -leg_foot);
        leg(x, leg_foot);
        // Cross brace.
        translate([x, 0, 0.5]) cube([leg_w, leg_foot * 1.1, 0.06], center = true);
    }
    // Foot rail in front, on short posts.
    translate([0, -foot_y, foot_h]) cube([len_x - 0.2, 0.12, 0.06], center = true);
    for (x = [-len_x / 2 + 0.1, len_x / 2 - 0.1])
        translate([x, -foot_y, foot_h / 2]) cube([leg_w, leg_w, foot_h], center = true);
}

// Slot dividers on the foot rail, one either side of each pair.
color(dark)
for (i = [0 : slot_n]) {
    x = (i - slot_n / 2) * slot_pitch;
    translate([x, -foot_y, foot_h + 0.06]) cube([0.03, 0.12, 0.08], center = true);
}

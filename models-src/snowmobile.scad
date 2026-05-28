// Snowmobile — ski-patrol rescue sled.
// A compact personal over-snow vehicle: single centred rear track, two
// front steering skis, a tapered hood over the engine, a seat, and
// handlebars. Much narrower and shorter than the snowcat.
//
// Real specs (Arctic Cat / Ski-Doo touring class):
//   length ~2.9 m  ·  width ~0.70 m  ·  height ~1.1 m to bar tops
//
// Body colours are near-white so the renderer's per-instance ColorTint
// controls the appearance cleanly: white tint = parked, red tint = active.
//
// Origin at the centre of the footprint on Z=0. +X is forward.
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 8;

// ── Dimensions (metres) ───────────────────────────────────────────────────

// Rear track — single centred belt
track_l = 2.50;
track_w = 0.42;
track_h = 0.22;

// Chassis / tunnel that everything mounts to
chassis_l = 2.55;
chassis_w = 0.62;
chassis_h = 0.26;

// Hood — tapers from the firewall (where chassis meets engine bay) to nose tip
hood_l      = 1.20; // measured forward from the firewall
hood_w_back = 0.60; // width at the firewall (matches chassis width)
hood_w_nose = 0.16; // width at the nose tip
hood_h      = 0.46; // hood height above chassis top

// Front skis — two wide-set steering skis; their centres live fore of
// the chassis nose so the tips visibly extend forward.
ski_l        = 1.05;
ski_w        = 0.13;
ski_h        = 0.06;
ski_span     = 0.38;    // half-span: ski centreline to vehicle centreline
ski_tip_fwd  = 0.30;    // how far the ski tip extends ahead of the chassis nose

// Seat — raised saddle slightly rearward of the chassis centre
seat_l = 0.80;
seat_w = 0.28;
seat_h = 0.16;
seat_x = -0.20;

// Windshield — raked slab ahead of the handlebars
ws_h   = 0.30;
ws_thk = 0.05;
ws_rake = 22; // degrees forward from vertical

// Handlebars — T-bar mounted above the windshield
hbar_w    = 0.64;
hbar_thk  = 0.04;
hbar_stem = 0.28; // stem height above chassis top

// ── Derived positions ─────────────────────────────────────────────────────
chassis_front = chassis_l / 2;
chassis_back  = -chassis_l / 2;
firewall_x    = chassis_front - hood_l; // where the hood starts

z_chassis = track_h;
z_top     = z_chassis + chassis_h;

ski_x = chassis_front + ski_tip_fwd - ski_l / 2;

hbar_x = firewall_x + 0.28; // handlebars live above the firewall area

// ── Modules ───────────────────────────────────────────────────────────────

module track() {
    color("DimGray")
        translate([0, 0, track_h / 2])
            cube([track_l, track_w, track_h], center = true);
    // Three cross-cleats suggest the tread pattern.
    color([0.35, 0.35, 0.35])
        for (dx = [-0.75, 0, 0.75])
            translate([dx, 0, track_h + 0.01])
                cube([0.14, track_w + 0.06, 0.03], center = true);
}

module chassis() {
    color([0.93, 0.93, 0.95])
        translate([0, 0, z_chassis + chassis_h / 2])
            cube([chassis_l, chassis_w, chassis_h], center = true);
}

// Hood: a convex hull from the wide firewall section to the narrow nose tip,
// creating a smooth taper that reads clearly as a snowmobile's engine cover.
module hood() {
    color([0.90, 0.90, 0.92])
        hull() {
            // Nose tip — very narrow
            translate([chassis_front + 0.06, 0, z_top + hood_h * 0.20])
                cube([0.06, hood_w_nose, hood_h * 0.40], center = true);
            // Firewall — full width, full height
            translate([firewall_x - 0.04, 0, z_top + hood_h * 0.50])
                cube([0.08, hood_w_back, hood_h], center = true);
        }
}

// One front steering ski. `side` = +1 (right) or −1 (left).
module ski(side) {
    // Skis are horizontal but with a slight tip upward at the front.
    tip_rise = 0.08;
    color([0.84, 0.84, 0.88])
        translate([ski_x, side * ski_span, ski_h / 2])
            rotate([0, -atan(tip_rise / ski_l), 0])
                cube([ski_l, ski_w, ski_h], center = true);
}

// Ski leg strut from chassis underside down to the ski
module ski_strut(side) {
    strut_bot = ski_h + 0.02;
    strut_top = z_chassis;
    strut_h   = strut_top - strut_bot;
    color("Gray")
        translate([ski_x + 0.12, side * ski_span, strut_bot + strut_h / 2])
            cube([0.08, 0.06, strut_h], center = true);
}

module seat() {
    color("DarkGray")
        translate([seat_x, 0, z_top + seat_h / 2])
            cube([seat_l, seat_w, seat_h], center = true);
}

module windshield() {
    color([0.75, 0.90, 0.95])
        translate([hbar_x + 0.10, 0, z_top + ws_h / 2])
            rotate([0, ws_rake, 0])
                cube([ws_thk, chassis_w * 0.85, ws_h], center = true);
}

module handlebars() {
    bar_z = z_top + hbar_stem;
    // Vertical stem
    color("Gray")
        translate([hbar_x, 0, z_top + hbar_stem / 2])
            cube([hbar_thk, hbar_thk, hbar_stem], center = true);
    // Horizontal crossbar
    color("Gray")
        translate([hbar_x, 0, bar_z])
            cube([hbar_thk * 2.5, hbar_w, hbar_thk], center = true);
    // Grip ends
    for (side = [-1, +1])
        color([0.20, 0.20, 0.20])
            translate([hbar_x, side * hbar_w / 2, bar_z])
                cube([0.10, 0.06, hbar_thk * 1.5], center = true);
}

module snowmobile() {
    track();
    chassis();
    hood();
    ski(+1);
    ski(-1);
    ski_strut(+1);
    ski_strut(-1);
    seat();
    windshield();
    handlebars();
}

snowmobile();

// Lift tower — chairlift tower. A tapered round steel pole on a concrete
// footing, a crossarm across the top, and a sheave train at each end
// carrying the cable, with a maintenance ladder up the back of the pole.
// Conventions, units, and axis docs: see models-src/README.md.
//
// The cable crosses the tower at cable_h (world.TowerHeight), cable_gap
// either side of the pole (world.CableGap). Chairs hang from the cable
// down to about 1.9 m below it and a quad's reaches to 0.3 m off the
// centreline, so the crossarm sits above the cable, the sheave wheels
// hang from it with the cable running along their undersides, and the
// pole narrows to clear the chairs' inner edge.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
cable_h   = 18.00; // world.TowerHeight
cable_gap = 1.50;  // world.CableGap

pole_r0   = 0.42;  // at the ground
pole_r1   = 0.24;  // at the crossarm
footing_r = 0.80;
footing_h = 0.45;  // above ground; it carries on below for sloped ground

sheave_r  = 0.18;
sheave_t  = 0.10;
sheaves   = 4;     // per train
sheave_dx = 0.55;  // spacing along the cable
train_len = sheaves * sheave_dx + 0.3;

arm_z     = cable_h + 2 * sheave_r + 0.22; // crossarm underside
arm_h     = 0.34;
arm_w     = 0.30;  // along the cable
arm_half  = cable_gap + 0.75;

ladder_x  = -(pole_r1 + 0.22); // up the back of the pole
ladder_w  = 0.40;
ladder_z0 = 2.4;               // out of reach from the ground
rung_dz   = 0.6;

// ── Geometry ───────────────────────────────────────────────────────────
module footing() {
    color("Silver")
        translate([0, 0, -1.5])
            cylinder(h = 1.5 + footing_h, r = footing_r);
}

module pole() {
    color("SlateGray")
        cylinder(h = arm_z + arm_h, r1 = pole_r0, r2 = pole_r1);
}

module crossarm() {
    color("SlateGray")
        translate([0, 0, arm_z + arm_h / 2])
            cube([arm_w, 2 * arm_half, arm_h], center = true);
    // A cap over the pole head.
    color("DimGray")
        translate([0, 0, arm_z + arm_h])
            cylinder(h = 0.12, r = pole_r1 + 0.08);
}

// A sheave train: wheels in a row along the cable, their bottoms on the
// cable, in a frame hung from the crossarm.
module sheave_train(y) {
    wheel_z = cable_h + sheave_r;
    color("DarkSlateGray")
        for (i = [0 : sheaves - 1])
            translate([(i - (sheaves - 1) / 2) * sheave_dx, y, wheel_z])
                rotate([90, 0, 0])
                    cylinder(h = sheave_t, r = sheave_r, center = true);
    // Side plates over the axles, and the hanger to the crossarm.
    color("Goldenrod") {
        for (s = [-1, 1])
            translate([0, y + s * (sheave_t / 2 + 0.04), wheel_z + 0.06])
                cube([train_len, 0.04, 0.16], center = true);
        translate([0, y, (wheel_z + arm_z) / 2 + 0.04])
            cube([0.22, 0.22, arm_z - wheel_z], center = true);
    }
}

module ladder() {
    color("DimGray") {
        for (s = [-1, 1])
            translate([ladder_x, s * ladder_w / 2, (ladder_z0 + arm_z) / 2])
                cube([0.05, 0.05, arm_z - ladder_z0], center = true);
        for (z = [ladder_z0 + 0.3 : rung_dz : arm_z - 0.2])
            translate([ladder_x, 0, z])
                cube([0.04, ladder_w, 0.04], center = true);
        // Stand-offs tying the rails to the pole.
        for (z = [ladder_z0 + 1, (ladder_z0 + arm_z) / 2, arm_z - 1])
            translate([(ladder_x - pole_r1) / 2, 0, z])
                cube([abs(ladder_x) - pole_r1 + 0.1, 0.05, 0.05], center = true);
    }
}

module tower() {
    footing();
    pole();
    crossarm();
    sheave_train(cable_gap);
    sheave_train(-cable_gap);
    ladder();
}

tower();

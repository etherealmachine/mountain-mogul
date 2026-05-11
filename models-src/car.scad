// Car — generic four-wheel passenger car, drawn per parked car by the
// dynamic car batch (see render.carInstancesFor). The body colour is
// supplied via per-instance ColorTint so a row of parked cars reads as
// a row of different-coloured vehicles; everything else (wheels, glass)
// uses its own SCAD colour that gets multiplied with the tint.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//   +X = forward (the car's front faces +X)
//   +Y = lateral (driver's left when facing +X)
//   +Z = up
//
// Sized for a generic sedan: 4.0 m long × 1.7 m wide × ~1.5 m tall.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
body_len = 4.00; // along X
body_w   = 1.70; // along Y
body_h   = 0.55; // lower body, from above the wheels to the windowsill

cabin_len = 2.20; // along X — sits roughly over the rear half
cabin_w   = 1.55; // slightly inset from body sides
cabin_h   = 0.55; // glass section above the body
cabin_x   = -0.30; // pulled back from centre so the hood is longer than the trunk

wheel_r        = 0.32;
wheel_thickness = 0.20;
wheelbase      = 2.50; // distance between front and rear axles along X
track          = 1.45; // distance between left and right wheels along Y

// Derived Z anchors
body_z  = wheel_r + 0.05;       // body sits just above the wheel tops
cabin_z = body_z + body_h;       // cabin sits on the body
ride_h  = body_z + body_h + cabin_h; // total car height to the roof

// ── Parts ──────────────────────────────────────────────────────────────
module wheel(x, y) {
    color([0.10, 0.10, 0.10]) // tyre black
    translate([x, y, wheel_r])
        rotate([90, 0, 0])
            cylinder(h = wheel_thickness, r = wheel_r, center = true);
}

module wheels() {
    wheel(+wheelbase / 2, -track / 2);
    wheel(+wheelbase / 2, +track / 2);
    wheel(-wheelbase / 2, -track / 2);
    wheel(-wheelbase / 2, +track / 2);
}

module body() {
    // White-ish in SCAD so the per-instance ColorTint can show the car's
    // colour. Per-vertex multiply gives (tint × 0.92), still readable.
    color([0.92, 0.92, 0.92])
    translate([0, 0, body_z + body_h / 2])
        cube([body_len, body_w, body_h], center = true);
}

module cabin() {
    // Glass cabin, tinted blue-grey. Multiplied with the per-instance
    // tint this lands in the dark-glass range regardless of car colour.
    color([0.30, 0.45, 0.55])
    translate([cabin_x, 0, cabin_z + cabin_h / 2])
        cube([cabin_len, cabin_w, cabin_h], center = true);
}

module headlights() {
    // Two small bright pads on the front face. Near-white so the
    // per-instance tint still leaves them noticeably brighter than the
    // body.
    color([1.0, 0.95, 0.80])
    for (y = [-body_w / 2 + 0.20, +body_w / 2 - 0.20])
        translate([body_len / 2 - 0.05, y, body_z + body_h * 0.45])
            cube([0.10, 0.20, 0.20], center = true);
}

module taillights() {
    color([0.85, 0.20, 0.20])
    for (y = [-body_w / 2 + 0.20, +body_w / 2 - 0.20])
        translate([-body_len / 2 + 0.05, y, body_z + body_h * 0.45])
            cube([0.10, 0.20, 0.20], center = true);
}

module car() {
    wheels();
    body();
    cabin();
    headlights();
    taillights();
}

car();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y). The car batch lays out a grid of
// instances inside each parking lot — the renderer doesn't currently
// read this footprint, but emitting it keeps the pipeline consistent
// with the building meshes and lets a future parking-lot density tuner
// query the canonical car size.
echo("MOGUL_META", "footprint", body_len / 2, body_w / 2);

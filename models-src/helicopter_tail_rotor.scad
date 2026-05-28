// Helicopter tail rotor — 2-blade assembly. Rendered as a separate animated
// part that spins around the game-Z axis (= SCAD Y = tail-boom shaft axis).
// Origin is at the rotor hub centre so the renderer can spin all vertices
// around (0,0,0).
//
// Keep dimensions in sync with helicopter_body.scad.

$fn = 10;

body_z    = 0.65;
body_h    = 2.0;
body_cx   = 0.8;
body_l    = 4.5;
body_rear = body_cx - body_l / 2;
boom_l    = 5.0;
boom_end_x = body_rear - boom_l;
boom_rise = 0.50;
body_top  = body_z + body_h;
boom_z_fwd = body_z + body_h * 0.38;
boom_z_end = boom_z_fwd + boom_rise;

fin_h   = 1.10;
fin_l   = 0.85;
fin_thk = 0.12;

tr_l = 0.80;
tr_w = 0.18;
tr_h = 0.05;
n_tr = 2;

// Hub position in SCAD space (matches helicopter_body.scad)
tr_x = boom_end_x + fin_l * 0.50;
tr_y = fin_thk / 2 + 0.06;
tr_z = boom_z_end + fin_h * 0.65;

// Translate so hub centre (tr_x, tr_y, tr_z) sits at origin.
translate([-tr_x, -tr_y, -tr_z]) {
    // Hub disc — axis along Y (shaft direction)
    color([0.32, 0.32, 0.35])
    translate([tr_x, tr_y, tr_z])
        rotate([90, 0, 0])
            cylinder(h = 0.10, r = 0.12, center = true);

    // Two blades, 180° apart, extending along X from hub centre
    color([0.12, 0.12, 0.14])
    for (i = [0 : n_tr - 1])
        translate([tr_x, tr_y, tr_z])
            rotate([0, i * 360 / n_tr, 0])
                translate([tr_l / 2, 0, 0])
                    cube([tr_l, tr_h, tr_w], center = true);
}

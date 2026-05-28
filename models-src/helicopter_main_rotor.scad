// Helicopter main rotor — 3-blade assembly. Rendered as a separate animated
// part that spins around the game-Y (vertical) axis. Origin is at the rotor
// hub centre so the renderer can spin all vertices around (0,0,0).
//
// Keep dimensions in sync with helicopter_body.scad.

$fn = 10;

body_cx = 0.8;
body_z  = 0.65;
body_h  = 2.0;
body_top = body_z + body_h;
mast_h  = 0.50;
mast_top = body_top + mast_h;

blade_l = 5.0;
blade_w = 0.30;
blade_h = 0.07;
n_main  = 3;

// Translate so the hub centre (body_cx, 0, mast_top) sits at origin.
translate([-body_cx, 0, -mast_top]) {
    // Hub disc
    color([0.32, 0.32, 0.35])
    translate([body_cx, 0, mast_top])
        cylinder(h = 0.14, r = 0.24, center = true);

    // Blades — each extends from hub centre outward in +X, rotated 120° apart
    color([0.12, 0.12, 0.14])
    for (i = [0 : n_main - 1])
        translate([body_cx, 0, mast_top + 0.08])
            rotate([0, 0, i * 360 / n_main])
                translate([blade_l / 2, 0, 0])
                    cube([blade_l, blade_w, blade_h], center = true);
}

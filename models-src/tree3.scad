// Conifer variant 2 — christmas-tree double cone. Lower truncated frustum
// stacked under an upper full cone; gives the forest one obviously-
// different silhouette so dense stands don't look like a single repeated
// stamp.
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 6;

trunk_r       = 0.18;
trunk_h       = 0.55;
lower_r       = 1.70;
lower_top_r   = 0.90;
lower_h       = 2.40;
upper_r       = 0.90;
upper_h       = 2.80;

module tree3() {
    color("SaddleBrown")
        cylinder(h = trunk_h, r = trunk_r);
    // Lower frustum (the broad skirt).
    color("DarkGreen")
        translate([0, 0, trunk_h])
            cylinder(h = lower_h, r1 = lower_r, r2 = lower_top_r);
    // Upper apex cone, perched on the frustum's top ring.
    color("DarkGreen")
        translate([0, 0, trunk_h + lower_h])
            cylinder(h = upper_h, r1 = upper_r, r2 = 0);
}

tree3();

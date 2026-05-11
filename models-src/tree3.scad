// Conifer variant 2 — christmas-tree double cone. Lower truncated frustum
// stacked under an upper full cone; gives the forest one obviously-
// different silhouette so dense stands don't look like a single repeated
// stamp.
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 6;

trunk_r       = 0.18;
trunk_h       = 1.50;
lower_r       = 1.70;
lower_top_r   = 0.90;
lower_h       = 2.40;
upper_r       = 0.90;
upper_h       = 2.80;

module tree3() {
    color("SaddleBrown")
        cylinder(h = trunk_h, r = trunk_r);
    // Foliage: blue-green fir — distinct from tree.scad / tree2.scad.
    color([0.30, 0.48, 0.36])
        translate([0, 0, trunk_h])
            cylinder(h = lower_h, r1 = lower_r, r2 = lower_top_r);
    color([0.30, 0.48, 0.36])
        translate([0, 0, trunk_h + lower_h])
            cylinder(h = upper_h, r1 = upper_r, r2 = 0);
}

tree3();

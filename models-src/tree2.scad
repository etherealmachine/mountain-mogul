// Conifer variant 1 — tall slender. Narrower canopy on a slightly taller
// trunk; reads as the "lodgepole pine" of the three variants.
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 6;

trunk_r   = 0.15;
trunk_h   = 0.70;
canopy_r  = 1.30;
canopy_h  = 6.00;

module tree2() {
    color("SaddleBrown")
        cylinder(h = trunk_h, r = trunk_r);
    color("DarkGreen")
        translate([0, 0, trunk_h])
            cylinder(h = canopy_h, r1 = canopy_r, r2 = 0);
}

tree2();

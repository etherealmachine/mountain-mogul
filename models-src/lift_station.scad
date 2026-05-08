// Lift station — base or top of a chairlift. Drawn in OpenSCAD-native
// Z-up convention; the stl2obj converter rotates to the game's Y-up axis
// at export time so this file reads naturally in the OpenSCAD preview.
//
// Origin is at the centre of the foundation, on the ground plane (z=0).
// Bullwheel housing sits offset toward +Y so we can later orient the
// model along the cable direction at placement time.

$fn = 16;  // segments per cylinder — keep low for game-suitable poly counts.

module lift_station() {
    // Concrete pad
    color("DimGray")
        translate([0, 0, 0.6])
            cube([6, 6, 1.2], center=true);

    // Four corner support pillars
    pillar_h = 4.0;
    color("LightSlateGray")
        for (sx = [-1, 1], sy = [-1, 1])
            translate([sx * 2.2, sy * 2.2, 1.2 + pillar_h/2])
                cube([0.6, 0.6, pillar_h], center=true);

    // Roof slab (slightly oversized so it overhangs)
    color("DarkSlateGray")
        translate([0, 0, 1.2 + pillar_h + 0.25])
            cube([6.2, 6.2, 0.5], center=true);

    // Bullwheel housing — cylinder with axis along Y. Cable runs through it.
    bw_z = 1.2 + pillar_h + 1.6;
    bw_r = 1.4;
    color("Goldenrod")
        translate([0, 0, bw_z])
            rotate([90, 0, 0])
                cylinder(h=2.4, r=bw_r, center=true);

    // Hub axle stubs poking out either side of the bullwheel
    color("DarkGray")
        for (sy = [-1, 1])
            translate([0, sy * 1.4, bw_z])
                rotate([90, 0, 0])
                    cylinder(h=0.6, r=0.4, center=true);
}

lift_station();

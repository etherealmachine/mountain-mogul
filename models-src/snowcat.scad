// Snowcat / piste basher — the over-snow tracked vehicle that grooms
// trails. Read at a glance: long parallel tracks, a tall cab forward
// of centre with a wraparound windshield, a V-shaped blade pushed
// out front for snow piling, and a wide tiller hanging off the back
// that does the actual corduroy work.
//
// Real specs (Pisten Bully 600 class):
//   length 7 m  ·  width with tracks 4 m  ·  height 3 m  ·  blade 5 m
// We dial the blade in a touch under that so the silhouette doesn't
// scream "monster truck" at low zoom.
//
// Origin at the centre of the footprint on Z = 0. +X is forward
// (direction of travel), +Y is the lateral axis.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 10;

// ── Dimensions (metres) ────────────────────────────────────────────────
hull_l = 5.0; // hull length (X) — tracks extend beyond this
hull_w = 2.4; // hull width (Y)
hull_h = 1.4; // hull thickness in Z (top of hull above ground)

// Tracks — running along ±Y, full length plus some overhang fore/aft.
track_l = 6.0; // longer than the hull so the cat reads as tracked, not wheeled
track_w = 0.7; // track belt width (Y)
track_h = 0.6; // track height (Z) — half-buried look at the deck level

// Cab — operator station above and slightly forward of the hull centre.
cab_l   = 2.0;
cab_w   = 2.0;
cab_h   = 1.4;
cab_x   = 0.2; // slight forward bias — operator sees the blade

// Cab roof — a thin slab cap
roof_thk = 0.08;

// Front blade — angled V-pusher mounted on a short arm out front.
blade_w = 4.4; // width across (Y)
blade_h = 1.2; // height (Z)
blade_thk = 0.15;
blade_v_offset = 0.6; // how far the centre of the V is pulled back vs the wings
arm_l = 0.8; // arm extending from the hull front to the blade
arm_w = 0.25;
arm_h = 0.25;

// Rear tiller — the wide drum-and-comb assembly that lays corduroy.
tiller_w = 3.6;
tiller_l = 0.9;
tiller_h = 0.5;
tiller_x = -hull_l / 2 - tiller_l / 2 - 0.1; // a touch behind the hull

// ── Geometry ───────────────────────────────────────────────────────────
module track(side) {
    // side = +1 (right) or -1 (left)
    track_y = side * (hull_w / 2 + track_w / 2 - 0.05);
    color("DimGray")
        translate([0, track_y, track_h / 2])
            cube([track_l, track_w, track_h], center = true);
}

module hull() {
    color("OrangeRed") // safety-orange like a real snowcat
        translate([0, 0, track_h + hull_h / 2])
            cube([hull_l, hull_w, hull_h], center = true);
}

module cab() {
    cab_z = track_h + hull_h + cab_h / 2;
    color("SteelBlue")
        translate([cab_x, 0, cab_z])
            cube([cab_l, cab_w, cab_h], center = true);
    // Roof slab — slightly oversized so it reads as a rain cap.
    color("DimGray")
        translate([cab_x, 0, track_h + hull_h + cab_h + roof_thk / 2])
            cube([cab_l + 0.15, cab_w + 0.15, roof_thk], center = true);
}

// Blade — a V whose wings angle forward of the centre. Built from two
// thin cuboids rotated about the V apex so each wing meets at the
// centreline.
module blade_wing(side) {
    wing_l = blade_w / 2;
    rotate([0, 0, side * -15]) // wings rake slightly back from the centre
        translate([0, side * wing_l / 2, blade_h / 2])
            cube([blade_thk, wing_l, blade_h], center = true);
}

module blade() {
    blade_x = hull_l / 2 + arm_l + blade_v_offset;
    // Support arm from hull front out to the blade centre
    color("DimGray")
        translate([hull_l / 2 + arm_l / 2, 0, track_h + arm_h / 2])
            cube([arm_l, arm_w, arm_h], center = true);

    color("Goldenrod")
        translate([blade_x, 0, track_h]) {
            blade_wing(+1);
            blade_wing(-1);
        }
}

// Tiller — the corduroy-laying assembly hanging off the back.
module tiller() {
    color("DimGray")
        translate([tiller_x, 0, tiller_h / 2])
            cube([tiller_l, tiller_w, tiller_h], center = true);
    // A few visible "fingers" suggesting the comb behind the drum.
    color("DarkGray")
        for (i = [-2 : 1 : 2])
            translate([tiller_x - tiller_l / 2 - 0.15, i * 0.7, tiller_h * 0.3])
                cube([0.30, 0.10, 0.10], center = true);
}

module snowcat() {
    track(+1);
    track(-1);
    hull();
    cab();
    blade();
    tiller();
}

snowcat();

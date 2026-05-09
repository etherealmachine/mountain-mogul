// Lift station — chairlift bottom / top station. A short column carries
// a long horizontal beam reaching back over a vertical-axis bullwheel.
// The column is intentionally low: skiers board at ground level, so the
// station's apparatus sits in the loading area rather than up at cable
// height. (Cables travel high between towers via world.CableHeight; how
// the cable transitions down to the bullwheel at each station is a
// separate visual problem.)
//
// Reference: ~/Downloads/lift.jpg (Norwegian T-bar bottom).
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
column_w = 0.70;  // cable-axis (X) face — narrow side
column_d = 1.40;  // perpendicular (Y) face — heftier
column_h = 4.00;  // top of column; just enough headroom for chairs to clear

beam_thick = 0.40; // vertical thickness (Z)
beam_wide  = 1.40; // perpendicular (Y) — matches column_d for visual continuity
beam_len   = 6.50; // long enough to put the bullwheel well clear of the column

// Bullwheel — vertical axis, horizontal disc.
bw_dia    = 3.66; // 12 ft, mid of the 10–15 ft real range
bw_thick  = 0.30; // 1 ft

// Axle stub from beam underside down to top of disc.
axle_len  = 0.20;
axle_r    = 0.18;

// ── Geometry ───────────────────────────────────────────────────────────
// Column: rises from origin in +Z. Origin is at the centre of its ground
// footprint, on the Z = 0 plane.
module column() {
    color("DimGray")
        translate([0, 0, column_h / 2])
            cube([column_w, column_d, column_h], center = true);
}

// Beam: extends from the top of the column toward the back (-X). Its
// bottom sits flush with the top of the column.
module beam() {
    color("LightSlateGray")
        translate([-beam_len / 2, 0, column_h + beam_thick / 2])
            cube([beam_len, beam_wide, beam_thick], center = true);
}

// Bullwheel: hangs from the underside of the beam at the -X end. Axis
// along Z (vertical) → wheel is a horizontal disc.
module bullwheel() {
    bw_x = -beam_len + bw_dia / 2 - 0.10; // a touch inboard of the beam end
    bw_z = column_h - axle_len - bw_thick / 2;

    // Axle stub
    color("DarkGray")
        translate([bw_x, 0, column_h - axle_len / 2])
            cylinder(h = axle_len, r = axle_r, center = true);

    // Wheel
    color("Goldenrod")
        translate([bw_x, 0, bw_z])
            cylinder(h = bw_thick, r = bw_dia / 2, center = true);
}

module lift_station() {
    column();
    beam();
    bullwheel();
}

lift_station();

// Lift station — simple surface-lift / fixed-grip bottom station.
// Reference: ~/Downloads/lift.jpg (Norwegian T-bar bottom).
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
column_w  = 0.30;  // 1 ft — narrow side, along cable axis (X)
column_d  = 0.91;  // 3 ft — wide side, perpendicular to cable (Y)
column_h  = 3.00;  // 10 ft — clears skiers underneath

beam_thick = 0.30; // 1 ft thick (Z)
beam_wide  = 0.91; // 3 ft wide (Y) — same profile as the column
beam_len   = 3.50; // long enough to clear the bullwheel radius

bw_dia    = 3.66;  // 12 ft — middle of the 10-15 ft real-world range
bw_thick  = 0.30;  // 1 ft thick
axle_len  = 0.20;  // short stub between beam underside and bullwheel top
axle_r    = 0.18;

// ── Geometry ───────────────────────────────────────────────────────────
// Column: rises from origin in +Z. Origin is at the centre of its
// ground footprint, on the Z=0 plane.
module column() {
    color("DimGray")
        translate([0, 0, column_h / 2])
            cube([column_w, column_d, column_h], center = true);
}

// Beam: extends from the top of the column toward the back (-X).
// Bottom of the beam sits flush with the top of the column.
module beam() {
    color("LightSlateGray")
        translate([-beam_len / 2, 0, column_h + beam_thick / 2])
            cube([beam_len, beam_wide, beam_thick], center = true);
}

// Bullwheel: hangs from the underside of the beam at the -X end.
// Axis along Z (vertical) → wheel is a horizontal disc.
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

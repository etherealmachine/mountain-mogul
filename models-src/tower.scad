// Lift tower — chairlift T-tower. One vertical pole with a horizontal
// crossbar at the top whose length matches the up/down cable spacing.
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
pole_w = 0.70;  // square cross-section, mid of the 0.6–0.9 m real range
pole_h = 18.00; // top of pole sits at cable height (matches world.TowerHeight)

bar_len    = 5.00; // perpendicular to cable; 2× world.CrossbarHalf
bar_thick  = 0.30; // along cable axis (X)
bar_height = 0.30; // vertical thickness

// ── Geometry ───────────────────────────────────────────────────────────
// Pole rises from the origin in +Z. Origin is the centre of the ground
// footprint on Z = 0.
module pole() {
    color("DimGray")
        translate([0, 0, pole_h / 2])
            cube([pole_w, pole_w, pole_h], center = true);
}

// Crossbar sits at the very top of the pole, flush with its top face.
// Long axis is +Y (perpendicular to cable axis), so cables hang from
// either end at ±bar_len/2.
module crossbar() {
    color("LightSlateGray")
        translate([0, 0, pole_h - bar_height / 2])
            cube([bar_thick, bar_len, bar_height], center = true);
}

module tower() {
    pole();
    crossbar();
}

tower();

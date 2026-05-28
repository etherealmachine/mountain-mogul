// Helipad — landing pad for the heli-ski helicopter.
// Concrete slab with classic yellow H and touchdown circle.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//   +X = helicopter approach axis
//   +Y = lateral
//   +Z = up
//
// Origin at Z = 0, centre of footprint.
// ICAO standard for light-helicopter pad: ≥ 15 m × 15 m.

$fn = 24;

// ── Dimensions (metres) ────────────────────────────────────────────────────

pad_size = 15.0;  // square pad (X and Y)
pad_thk  =  0.12; // concrete slab thickness

mark_thk =  0.02; // painted-marking thickness
mark_z   = pad_thk + 0.005; // marking surface sits just above slab

// Touchdown circle (ring)
circ_ro = 5.8;  // outer radius
circ_ri = 5.2;  // inner radius (ring width 0.6 m)

// H marking
H_len  = 5.2;   // leg length (X)
H_span = 4.0;   // outer leg span (Y)
H_bar  = 0.75;  // width of each bar

// ── Derived ────────────────────────────────────────────────────────────────
H_leg_y  = H_span / 2 - H_bar / 2;        // leg centre Y offset from origin
H_xbar_w = H_span - 2 * H_bar;            // crossbar Y extent (between inner faces)

// ── Modules ────────────────────────────────────────────────────────────────

module slab() {
    color([0.20, 0.20, 0.22]) // dark asphalt
    translate([0, 0, pad_thk / 2])
        cube([pad_size, pad_size, pad_thk], center = true);
}

module circle_marking() {
    color([0.92, 0.82, 0.05]) // safety yellow
    translate([0, 0, mark_z])
        difference() {
            cylinder(h = mark_thk, r = circ_ro);
            // Subtract inner cylinder — leaves a ring
            translate([0, 0, -0.01])
                cylinder(h = mark_thk + 0.02, r = circ_ri);
        }
}

module H_marking() {
    color([0.92, 0.82, 0.05])
    translate([0, 0, mark_z]) {
        // Left leg
        translate([0, -H_leg_y, mark_thk / 2])
            cube([H_len, H_bar, mark_thk], center = true);
        // Right leg
        translate([0, +H_leg_y, mark_thk / 2])
            cube([H_len, H_bar, mark_thk], center = true);
        // Crossbar
        translate([0, 0, mark_thk / 2])
            cube([H_bar, H_xbar_w, mark_thk], center = true);
    }
}

module helipad() {
    slab();
    circle_marking();
    H_marking();
}

echo("MOGUL_META", "footprint", pad_size / 2, pad_size / 2);

helipad();

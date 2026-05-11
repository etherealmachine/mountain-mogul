// Lodge / base building — single-storey day lodge. Rectangular plan
// with a steep gable roof so snow sheds off rather than piling up.
//
// Sized relative to the rest of the world: a skier is ~1.7 m tall, a
// lift tower is 18 m, so the lodge eaves at 5 m and the ridge at 12 m
// reads as a substantial-but-not-monumental building.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
wall_w = 18.0; // long side, +X primary direction
wall_d = 12.0; // short side, +Y perpendicular
wall_h = 5.0;  // wall height before the gable starts

roof_rise = 7.0;  // ridge height above the wall top — gives ~49° pitch
overhang  = 0.6;  // eaves overhang on every side

// ── Geometry ───────────────────────────────────────────────────────────
// Walls: rectangular box centred on the origin, sitting on Z = 0.
module walls() {
    color("BurlyWood")
        translate([0, 0, wall_h / 2])
            cube([wall_w, wall_d, wall_h], center = true);
}

// Roof: triangular prism running along the long axis (X). linear_extrude
// with scale = [1, 0] tapers Y to zero at the ridge while keeping X
// full-length, so the cross-section in YZ is a triangle whose apex
// (the ridge line) runs along X. The base is slightly larger than the
// walls so the eaves overhang.
module roof() {
    color("DarkSlateGray")
        translate([0, 0, wall_h])
            linear_extrude(height = roof_rise, scale = [1, 0])
                square([wall_w + 2*overhang, wall_d + 2*overhang], center = true);
}

module lodge() {
    walls();
    roof();
}

lodge();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y) — the eaves overhang adds 0.6 m
// per side, which we include so the apron + tree-clearance zone covers
// the visible roof outline rather than just the wall plane. The
// scad2obj converter applies the SCAD → game rotation and writes the
// canonical (halfX, halfZ) into the OBJ header for the placement pass.
echo("MOGUL_META", "footprint", wall_w / 2 + overhang, wall_d / 2 + overhang);

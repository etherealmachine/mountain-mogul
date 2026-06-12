// Bar — compact lodge-style building, roughly half-size of the main lodge.
// Rectangular plan with a steep gable roof so snow sheds off rather than
// piling up. Designed for casual gathering: a warm spot near the lifts
// without the scale of the full lodge.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = primary horizontal direction (front façade faces +X)
//     +Y = lateral / perpendicular horizontal
//     +Z = up in SCAD (rotates to game Y-up at export)
//   Origin sits at ground plane, centred on the footprint.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
// Roughly half-size of the lodge for a secondary/annex scale:
//   Lodge: 18 m × 12 m → Bar: 9.0 m × 6.0 m
bar_w = 9.0;    // long side, +X primary direction (front faces +X)
bar_d = 6.0;    // short side, +Y perpendicular
bar_h = 4.5;    // wall height before the gable starts

// Steep ~48° pitch so snow sheds cleanly.
roof_rise = 6.75;      // ridge height above wall top
overhang = 0.5;        // eaves overhang on every side

// ── Geometry ───────────────────────────────────────────────────────────
// Walls: rectangular box centred on the origin, sitting on Z = 0.
module walls() {
    color("Tan")
        translate([0, 0, bar_h / 2])
            cube([bar_w, bar_d, bar_h], center = true);
}

// Roof: triangular prism running along the long axis (X). linear_extrude
// with scale = [1, 0] tapers Y to zero at the ridge while keeping X
// full-length. The base is slightly larger than the walls so the eaves
// overhang all sides.
module roof() {
    color("DarkSlateGray")
        translate([0, 0, bar_h])
            linear_extrude(height = roof_rise, scale = [1, 0])
                square([bar_w + 2*overhang, bar_d + 2*overhang], center = true);
}

module bar() {
    walls();
    roof();
}

bar();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y) — the eaves overhang adds 0.5 m
// per side so the apron + tree-clearance zone covers the full visible
// silhouette including the roof edge. The scad2obj converter applies the
// SCAD → game rotation and writes the canonical (halfX, halfZ) into the
// OBJ header for the placement pass.
echo("MOGUL_META", "footprint", bar_w / 2 + overhang, bar_d / 2 + overhang);

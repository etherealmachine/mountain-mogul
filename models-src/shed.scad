// Equipment shed — garage-style outbuilding for snowcats and snowmobiles.
// Reads as utilitarian: wider and lower than a lodge, with a shallow
// gable, prominent roll-up bay doors on the front (+X) for driving
// equipment in and out, and no decorative roof overhang.
//
// Sized for two snowcat bays (each cat is ~4 m wide × ~7 m long with
// blade) plus circulation space — 16 m × 12 m on the ground with
// 5 m wall height (clears a cat with the blade raised). Real snow-cat
// barns can be 25-30 m wide but a smaller building reads cleaner at
// game scale.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
wall_w = 16.0; // long side, +X primary direction (door faces +X)
wall_d = 12.0; // short side, +Y perpendicular
wall_h = 5.0;  // wall height before the gable starts

roof_rise = 2.5; // shallow ~17° pitch — utilitarian, not residential

// Door geometry — two bays on the +X wall.
door_w     = 5.0; // each bay's width (Y axis on the front face)
door_h     = 4.0; // tall enough to clear a cat with its blade up
door_gap   = 1.0; // gap between the two doors
door_inset = 0.05; // recess depth so the door reads as a panel

// Office annex — a small bump-out on the −X back wall, suggesting
// where the foreman's desk and parts shelf live.
office_w = 4.0;
office_d = 3.0;
office_h = 3.0;

// ── Geometry ───────────────────────────────────────────────────────────
// Main hall — rectangular box centred on the origin, sitting on Z = 0.
module hall() {
    color("LightSteelBlue")
        translate([0, 0, wall_h / 2])
            cube([wall_w, wall_d, wall_h], center = true);
}

// Roof: triangular prism running along the long axis. Same scale trick
// as the lodge but with no overhang, so the building reads industrial.
module roof() {
    color("DimGray")
        translate([0, 0, wall_h])
            linear_extrude(height = roof_rise, scale = [1, 0])
                square([wall_w, wall_d], center = true);
}

// Two bay doors on the +X face. Each door is a thin panel inset into
// the wall by `door_inset` so it reads as a recessed door rather than
// a paint mark.
module bay_door(y_centre) {
    color("Goldenrod")
        translate([wall_w / 2 - door_inset / 2, y_centre, door_h / 2])
            cube([door_inset, door_w, door_h], center = true);
}

module bay_doors() {
    // Two doors centred on the front, separated by door_gap.
    half_span = door_w / 2 + door_gap / 2;
    bay_door(-half_span);
    bay_door(+half_span);
}

// Office annex on the −X back wall.
module office() {
    color("Tan")
        translate([-wall_w / 2 - office_w / 2, 0, office_h / 2])
            cube([office_w, office_d, office_h], center = true);
}

// A short flat-roof cap for the office so the gabled main roof doesn't
// have to extend over the bump-out.
module office_roof() {
    color("DimGray")
        translate([-wall_w / 2 - office_w / 2, 0, office_h + 0.1])
            cube([office_w, office_d, 0.2], center = true);
}

module shed() {
    hall();
    roof();
    bay_doors();
    office();
    office_roof();
}

shed();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y). The office annex sticks out the
// −X back wall by office_w, so the full X extent is (wall_w/2 + office_w)
// on the back side and wall_w/2 on the front side — we use the larger of
// the two so the apron covers the full silhouette. The scad2obj converter
// applies the SCAD → game rotation and writes the canonical (halfX, halfZ)
// into the OBJ header for the placement pass.
echo("MOGUL_META", "footprint", wall_w / 2 + office_w, wall_d / 2);

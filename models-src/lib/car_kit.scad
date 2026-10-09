// Car kit — the parts every road vehicle shares (car_sedan.scad,
// car_minisuv.scad, car_suv.scad, car_jeep.scad, car_van.scad), which
// differ in their proportions. world.CarKind names them; the renderer
// draws each kind from its own mesh, tinted per car, with a roof rack or
// box (car_roof_rack.scad, car_roof_box.scad) set on the roof at the
// height in render.carRoofs.
//
//   +X = forward (the car's front faces +X)
//   +Y = lateral (driver's left when facing +X)
//   +Z = up
//
// The paint is near white so the per-car tint is the paint colour;
// glass, tyres, trim and lights are dark or bright enough to read
// through any tint.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 8;

paint  = [0.92, 0.92, 0.92];
glass  = [0.30, 0.40, 0.50];
trim   = [0.16, 0.16, 0.17]; // bumpers, grille, rails, mirrors
tyre   = [0.08, 0.08, 0.08];
hubcap = [0.55, 0.56, 0.58];
lamp   = [1.00, 0.96, 0.82];
tail   = [0.80, 0.12, 0.10];

// slab is a thin box from x0 to x1, centred in Y, its top at z.
module slab(x0, x1, w, z, t = 0.04) {
    translate([x0, -w / 2, z - t]) cube([x1 - x0, w, t]);
}

module wheel(x, y, r, t) {
    translate([x, y, r]) rotate([90, 0, 0]) {
        color(tyre) cylinder(h = t, r = r, center = true);
        color(hubcap) cylinder(h = t + 0.02, r = r * 0.55, center = true);
    }
}

// vehicle draws one car.
//   len, w        overall body length and width
//   r, wb         wheel radius and wheelbase (axles centred on x = axle_x)
//   sill          height of the body's underside
//   belt          the beltline: top of the doors, bottom of the glass
//   hood          top of the hood at the nose (below belt slopes it)
//   nose, tailin  how far the top of the body is drawn in from the
//                 front and back ends (rounding them)
//   roof          roof height
//   gb            [x0, x1] the glass at the beltline
//   gr            [x0, x1] the glass at the roof
//   rails         roof rails (SUVs)
module vehicle(len, w, r, wb, sill, belt, hood, nose, tailin, roof, gb, gr,
               axle_x = 0, rails = false) {
    // Body: a hull from the full-length underside to the beltline at the
    // back and the hood at the nose.
    color(paint) hull() {
        slab(-len / 2, len / 2, w, sill + 0.12, 0.12);
        slab(-len / 2 + tailin, gb[0] + 0.1, w, belt);
        slab(gb[1] - 0.1, len / 2 - nose, w - 0.06, hood);
        slab(-len / 2, -len / 2 + 0.05, w - 0.04, belt - 0.12);
        slab(len / 2 - 0.05, len / 2, w - 0.08, hood - 0.12);
    }
    // Glass greenhouse, and the painted roof on it.
    color(glass) hull() {
        slab(gb[0], gb[1], w - 0.10, belt + 0.02, 0.04);
        slab(gr[0], gr[1], w - 0.26, roof - 0.04, 0.04);
    }
    color(paint) slab(gr[0] - 0.03, gr[1] + 0.03, w - 0.22, roof, 0.06);
    // Pillars down the sides of the glass: the windshield's (A), between
    // the doors (B), and a broad one at the back (C).
    color(paint) for (s = [-1, 1]) {
        pillar(gb[1] - 0.14, gb[1], gr[1] - 0.10, gr[1], s);
        pillar((gb[0] + gb[1]) / 2 - 0.06, (gb[0] + gb[1]) / 2 + 0.06,
               (gr[0] + gr[1]) / 2 - 0.06, (gr[0] + gr[1]) / 2 + 0.06, s);
        pillar(gb[0], gb[0] + 0.30, gr[0], gr[0] + 0.24, s);
    }
    module pillar(b0, b1, r0, r1, s) hull() {
        translate([b0, s * (w - 0.10) / 2 - 0.03, belt]) cube([b1 - b0, 0.06, 0.02]);
        translate([r0, s * (w - 0.26) / 2 - 0.03, roof - 0.06]) cube([r1 - r0, 0.06, 0.02]);
    }
    if (rails)
        color(trim) for (s = [-1, 1])
            translate([gr[0] + 0.05, s * (w / 2 - 0.2) - 0.03, roof])
                cube([gr[1] - gr[0] - 0.1, 0.06, 0.06]);

    // Wheels, flush with the body sides.
    t = 0.24;
    for (x = [axle_x + wb / 2, axle_x - wb / 2], s = [-1, 1])
        wheel(x, s * (w / 2 - t / 2 + 0.02), r, t);

    // Bumpers, grille, lights.
    bz = sill + 0.02;
    color(trim) {
        translate([len / 2 - 0.08, -w / 2 + 0.02, bz]) cube([0.14, w - 0.04, 0.22]);
        translate([-len / 2 - 0.06, -w / 2 + 0.02, bz]) cube([0.14, w - 0.04, 0.22]);
        translate([len / 2 - 0.03, -w * 0.25, bz + 0.24]) cube([0.05, w * 0.5, max(hood - bz - 0.42, 0.12)]);
    }
    lz = hood - 0.16;
    color(lamp) for (s = [-1, 1])
        translate([len / 2 - 0.04, s * (w / 2 - 0.22) - 0.14, lz - 0.08]) cube([0.06, 0.28, 0.13]);
    color(tail) for (s = [-1, 1])
        translate([-len / 2 - 0.02, s * (w / 2 - 0.16) - 0.10, belt - 0.30]) cube([0.06, 0.20, 0.16]);
    // Mirrors.
    color(trim) for (s = [-1, 1])
        translate([gb[1] - 0.25, s * (w / 2 + 0.06) - 0.06, belt + 0.02]) cube([0.12, 0.12, 0.10]);

    echo("MOGUL_META", "footprint", len / 2, w / 2 + 0.08);
}

// Roof box — a long streamlined cargo pod on crossbars, set on a car's
// roof (render.carRoofs gives each kind's roof height). Origin at its
// centre on the roof. Near white, tinted per car (black, grey, silver
// or white).
// Conventions, units, and axis docs: see models-src/README.md.
$fn = 12;

color([0.10, 0.10, 0.10]) for (x = [-0.45, 0.45])
    translate([x - 0.03, -0.70, 0]) cube([0.06, 1.40, 0.07]);
color([0.92, 0.92, 0.92]) hull() {
    translate([-0.90, -0.40, 0.07]) cube([1.55, 0.80, 0.10]);
    translate([-0.80, -0.36, 0.07]) cube([1.30, 0.72, 0.38]);
    translate([0.95, -0.18, 0.10]) cube([0.04, 0.36, 0.14]);
}

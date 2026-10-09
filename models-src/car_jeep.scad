// Jeep — a short, boxy off-roader: upright windshield, flat hood, chunky
// tyres, a spare wheel on the tailgate. Seats four. Parts and
// conventions: lib/car_kit.scad.
include <lib/car_kit.scad>

len = 4.10;
vehicle(len = len, w = 1.86, r = 0.42, wb = 2.45, sill = 0.40,
        belt = 1.12, hood = 1.08, nose = 0.04, tailin = 0.02,
        roof = 1.86, gb = [-1.95, 0.55], gr = [-1.95, 0.48]);

// Spare wheel on the tailgate.
translate([-len / 2 - 0.16, 0, 0.95]) rotate([0, 90, 0]) {
    color(tyre) cylinder(h = 0.22, r = 0.38, center = true);
    color(hubcap) cylinder(h = 0.24, r = 0.18, center = true);
}
// Grille slots across the nose.
color(trim) for (i = [-3:3])
    translate([len / 2 - 0.02, i * 0.12 - 0.03, 0.70]) cube([0.04, 0.06, 0.26]);

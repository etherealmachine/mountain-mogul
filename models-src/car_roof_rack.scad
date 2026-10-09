// Roof rack with skis — two crossbars and three pairs of skis clamped
// tips forward, set on a car's roof (render.carRoofs gives each kind's
// roof height). Origin at the rack's centre on the roof. Drawn with a
// white tint, so the skis keep their colours.
// Conventions, units, and axis docs: see models-src/README.md.
$fn = 8;

bar_w = 1.50;
color([0.12, 0.12, 0.12]) for (x = [-0.45, 0.45]) {
    translate([x - 0.03, -bar_w / 2, 0]) cube([0.06, bar_w, 0.07]);
    for (s = [-1, 1]) translate([x - 0.06, s * (bar_w / 2 - 0.05) - 0.05, 0]) cube([0.12, 0.10, 0.10]);
}
skis = [[0.85, 0.15, 0.12], [0.15, 0.40, 0.85], [0.95, 0.85, 0.20]];
for (i = [0:2]) color(skis[i]) for (d = [-0.05, 0.05])
    translate([-0.85, (i - 1) * 0.42 + d - 0.035, 0.07]) {
        cube([1.62, 0.07, 0.025]);
        translate([1.62, 0, 0]) rotate([0, -25, 0]) cube([0.12, 0.07, 0.02]); // tip
    }
// Clamps over the skis.
color([0.12, 0.12, 0.12]) for (x = [-0.45, 0.45])
    translate([x - 0.04, -bar_w / 2 + 0.1, 0.095]) cube([0.08, bar_w - 0.2, 0.03]);

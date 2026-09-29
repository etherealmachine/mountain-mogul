// Ticket office — the day-ticket and season-pass window at the base area.
// A small timber booth, long across its front so several guests can be
// served side by side: three service windows on the +X façade under a
// red awning, a counter ledge beneath them, and a sign board on the roof
// so the building reads as "tickets" from the top-down camera.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//     +X = front façade (service windows face +X)
//     +Y = lateral / perpendicular horizontal
//     +Z = up in SCAD (rotates to game Y-up at export)
//   Origin sits at ground plane, centred on the footprint.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
booth_d = 4.0; // depth, front to back along X
booth_w = 8.0; // width along Y — the service frontage
booth_h = 3.2; // wall height at the front

// Mono-pitch roof falling toward the back so snow slides off away from
// the queue.
roof_drop     = 0.8;
roof_overhang = 0.4;
roof_t        = 0.25;

// Service windows on the front face.
window_count = 3;
window_w     = 1.6;
window_h     = 1.2;
window_sill  = 1.1; // counter height
window_inset = 0.05;

// Counter ledge and awning over the windows.
ledge_depth   = 0.35;
awning_depth  = 1.2;
awning_height = 2.55;
awning_t      = 0.12;

// Roof sign.
sign_w = 5.0;
sign_h = 1.1;
sign_t = 0.2;

// ── Geometry ───────────────────────────────────────────────────────────
// Walls: side profile (front booth_h, back roof_drop lower) extruded
// across the width, so the wall tops follow the roof pitch.
module walls() {
    color("Sienna")
        rotate([90, 0, 0])
            linear_extrude(height = booth_w, center = true)
                polygon([
                    [-booth_d / 2, 0],
                    [ booth_d / 2, 0],
                    [ booth_d / 2, booth_h],
                    [-booth_d / 2, booth_h - roof_drop],
                ]);
}

// Mono-pitch slab: a thin box tilted about Y so its underside meets the
// wall tops, high at the front and falling to the back.
module roof() {
    run   = booth_d + 2 * roof_overhang;
    angle = atan(roof_drop / booth_d);
    color("DarkSlateGray")
        translate([0, 0, booth_h - roof_drop / 2 + roof_t / 2])
            rotate([0, -angle, 0])
                cube([run / cos(angle), booth_w + 2 * roof_overhang, roof_t], center = true);
}

function window_y(i) = (i - (window_count - 1) / 2) * (booth_w / window_count);

module windows() {
    for (i = [0 : window_count - 1]) {
        color("LightSkyBlue")
            translate([booth_d / 2 + window_inset / 2, window_y(i), window_sill + window_h / 2])
                cube([window_inset, window_w, window_h], center = true);
    }
}

module ledge() {
    color("Tan")
        translate([booth_d / 2 + ledge_depth / 2, 0, window_sill - 0.05])
            cube([ledge_depth, booth_w - 0.6, 0.1], center = true);
}

module awning() {
    color("FireBrick")
        translate([booth_d / 2 + awning_depth / 2, 0, awning_height])
            rotate([0, 12, 0])
                cube([awning_depth, booth_w - 0.3, awning_t], center = true);
}

// Sign board on posts along the roof's front edge, facing +X.
module sign() {
    post = 0.15;
    base = booth_h - 0.2; // posts start inside the sloped roof slab
    lift = 0.8;
    color("DimGray")
        for (y = [-sign_w / 2 + 0.4, sign_w / 2 - 0.4])
            translate([booth_d / 2 - 0.6, y, base + lift / 2])
                cube([post, post, lift], center = true);
    color("Gold")
        translate([booth_d / 2 - 0.6, 0, base + lift + sign_h / 2])
            cube([sign_t, sign_w, sign_h], center = true);
}

module ticket_office() {
    walls();
    roof();
    windows();
    ledge();
    awning();
    sign();
}

ticket_office();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y). The awning projects awning_depth
// past the front wall; use that side for halfX so the apron covers it.
echo("MOGUL_META", "footprint", booth_d / 2 + awning_depth, booth_w / 2 + roof_overhang);

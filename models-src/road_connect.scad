// Road edge-connection marker — a yellow post with a small flag, placed
// by the scenario editor to mark where roads meet the map perimeter (cars
// spawn / despawn here at runtime). Visible in both editor and scenario
// so designers can see them while authoring and players can read where
// the world "ends" for the road network.
//
// Tall enough to read against the road at the standard zoomed-out camera
// distance; thin enough not to dominate the scene visually. Black post +
// yellow flag echo real-world boundary-marker semantics.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
post_w = 0.30;   // square section
post_h = 2.50;

flag_thick = 0.05; // along the post's X face
flag_len   = 1.20; // length the flag extends along +Y
flag_h     = 0.50;
flag_z0    = post_h - flag_h - 0.10; // top of flag a touch below post tip

// ── Geometry ───────────────────────────────────────────────────────────
// Post rises from the origin in +Z. Origin is the centre of the ground
// footprint on Z = 0.
module post() {
    color("Black")
        translate([0, 0, post_h / 2])
            cube([post_w, post_w, post_h], center = true);
}

// Flag hinges off the +X face of the post and extends in +Y. The high
// visibility comes from the colour, not the size — the flag stays small
// so the marker reads as "infrastructure", not "monument".
module flag() {
    color("Yellow")
        translate([post_w / 2, flag_len / 2, flag_z0 + flag_h / 2])
            cube([flag_thick, flag_len, flag_h], center = true);
}

module road_connect() {
    post();
    flag();
}

road_connect();

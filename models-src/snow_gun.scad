// Snow gun — industrial snowmaking cannon on a tripod mount.
//
// A narrow tripod base spreads three legs outward around a central hub.
// A vertical swivel post rises from the hub, and a horizontal cannon
// barrel points along +X (the direction the nozzle faces). The game
// spreads snow in a uniform circle regardless of barrel orientation, but
// the directional silhouette reads clearly at resort scale.
//
// Real snow guns are 0.5–1 m in diameter and 1–2 m long; this model
// is sized to be visible at the same zoom level as a patrol hut without
// dominating the terrain.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
// Tripod
leg_len    = 0.9;  // leg length from hub centre
leg_w      = 0.08; // leg cross-section (square)
hub_r      = 0.22; // hub cylinder radius
hub_h      = 0.35; // hub cylinder height

// Swivel post
post_r     = 0.12;
post_h     = 0.75;

// Cannon barrel (horizontal, angled slightly up)
barrel_r   = 0.11;
barrel_len = 1.1;
barrel_tilt = 12;  // degrees above horizontal

// Nozzle shroud (cone at the barrel tip)
nozzle_r   = 0.22; // outer radius at the open end
nozzle_h   = 0.18;

// ── Geometry ───────────────────────────────────────────────────────────

// Hub cylinder sitting on the ground.
module hub() {
    color("DimGray")
        cylinder(r = hub_r, h = hub_h, center = false);
}

// One tripod leg — a thin rectangular prism angled outward in the XY plane.
// `angle` rotates the leg around Z so three legs splay 120° apart.
module leg(angle) {
    color("DimGray")
        rotate([0, 0, angle])
            translate([leg_len / 2, 0, leg_w / 2])
                cube([leg_len, leg_w, leg_w], center = true);
}

module tripod() {
    hub();
    leg(0);
    leg(120);
    leg(240);
}

// Swivel post rising from the top of the hub.
module post() {
    color("DimGray")
        translate([0, 0, hub_h])
            cylinder(r = post_r, h = post_h, center = false);
}

// Cannon barrel: horizontal cylinder on the swivel post, angled `barrel_tilt`
// degrees above horizontal and pointing along +X.
module barrel() {
    base_z = hub_h + post_h;
    color("SlateGray")
        translate([0, 0, base_z])
            rotate([0, -barrel_tilt, 0])
                translate([barrel_len / 2, 0, 0])
                    rotate([0, 90, 0])
                        cylinder(r = barrel_r, h = barrel_len, center = true);
}

// Nozzle shroud — a cone at the +X tip of the barrel suggesting the fan-spray outlet.
module nozzle() {
    base_z  = hub_h + post_h;
    tip_x   = barrel_len * cos(barrel_tilt);
    tip_z   = barrel_len * sin(barrel_tilt);
    color("SlateGray")
        translate([tip_x, 0, base_z + tip_z])
            rotate([0, -barrel_tilt, 0])
                rotate([0, 90, 0])
                    cylinder(r1 = barrel_r, r2 = nozzle_r, h = nozzle_h, center = false);
}

tripod();
post();
barrel();
nozzle();


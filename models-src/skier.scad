// Skier — stylised standing figure on skis. One instance is drawn per
// world.Agent every frame via the dynamic batch; the shader rotates
// around world Y so the +X (forward) axis aligns with the agent's
// heading. The mesh is therefore authored as a generic skier facing
// down the +X axis.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//   +X = forward (direction the skier is travelling)
//   +Y = lateral (skier's left when facing +X)
//   +Z = up
//
// Sized for a 1.7 m tall adult on a pair of 1.7 m skis. The pose is a
// neutral upright stance — bent knees and forward lean look better but
// add joints the simple primitive shapes can't easily express, and at
// game scale the skier reads as "a person on skis" either way.
//
// Per-vertex colours are NOT kept near-white: the dynamic batch's
// ColorTint is used to convey activity state (Walking / Queuing /
// On Lift / etc., see agentColor in render/renderer.go), so each
// part has its own muted colour that the per-instance tint can darken
// or lighten on top.

$fn = 12;

// ── Dimensions (metres) ────────────────────────────────────────────────
ski_len = 1.70; // typical adult length
ski_w   = 0.10; // ski width (along Y)
ski_h   = 0.03; // ski thickness
ski_gap = 0.16; // half the distance between the two skis along Y

boot_len = 0.32; // along X
boot_w   = 0.12; // along Y, slightly wider than the ski
boot_h   = 0.22; // ankle-height

leg_r = 0.10;
leg_h = 0.55; // boot top → hip

torso_d = 0.30; // along X (chest depth)
torso_w = 0.46; // along Y (shoulder width)
torso_h = 0.55; // hip → top of shoulders

head_r = 0.13;

arm_r       = 0.07;
arm_h       = 0.55;
arm_shoulder_y = torso_w / 2 - arm_r; // arms hang from shoulder edge

// Derived Z coordinates (everything stacks on top of the skis).
ski_top   = ski_h;
boot_top  = ski_top + boot_h;
leg_top   = boot_top + leg_h;
torso_top = leg_top + torso_h;

// ── Parts ──────────────────────────────────────────────────────────────
module skis() {
    color([0.18, 0.20, 0.24]) // dark slate
    for (y = [-ski_gap, +ski_gap])
        translate([0, y, ski_h / 2])
            cube([ski_len, ski_w, ski_h], center = true);
}

module boots() {
    color([0.10, 0.10, 0.12])
    for (y = [-ski_gap, +ski_gap])
        translate([0, y, ski_top + boot_h / 2])
            cube([boot_len, boot_w, boot_h], center = true);
}

module legs() {
    color([0.20, 0.22, 0.30]) // dark pants
    for (y = [-ski_gap, +ski_gap])
        translate([0, y, boot_top + leg_h / 2])
            cylinder(h = leg_h, r = leg_r, center = true);
}

module torso() {
    color([0.85, 0.30, 0.30]) // ski-jacket red — readable from above
    translate([0, 0, leg_top + torso_h / 2])
        cube([torso_d, torso_w, torso_h], center = true);
}

module arms() {
    color([0.85, 0.30, 0.30]) // matches jacket
    for (y = [-arm_shoulder_y, +arm_shoulder_y])
        translate([0, y, leg_top + torso_h - arm_h / 2])
            cylinder(h = arm_h, r = arm_r, center = true);
}

module head() {
    color([0.95, 0.80, 0.65]) // fair skin
    translate([0, 0, torso_top + head_r])
        sphere(r = head_r);
}

module helmet() {
    color([0.15, 0.15, 0.18])
    translate([0, 0, torso_top + head_r])
        // Half-sphere covering the top of the head.
        difference() {
            sphere(r = head_r + 0.015);
            translate([0, 0, -head_r])
                cube([head_r * 3, head_r * 3, head_r * 2], center = true);
        }
}

module skier() {
    skis();
    boots();
    legs();
    torso();
    arms();
    head();
    helmet();
}

skier();

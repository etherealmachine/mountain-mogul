// Walker — same stylised figure as skier.scad but without skis.
// Used for guests who have removed their skis to cross a building
// footprint or bare-ground patch.
//
// Same axis conventions as skier.scad:
//   +X = forward (direction of travel), +Y = lateral (left), +Z = up
//
// Legs start at Z = 0 (ground) instead of ski_top + boot_h.

$fn = 12;

// ── Dimensions (metres) ───────────────────────────────────────────────────
shoe_len = 0.28;
shoe_w   = 0.12;
shoe_h   = 0.08;
shoe_gap = 0.14; // half-distance between feet (narrower than ski stance)

leg_r = 0.10;
leg_h = 0.55;

torso_d = 0.30;
torso_w = 0.46;
torso_h = 0.55;

head_r = 0.13;

arm_r          = 0.07;
arm_h          = 0.55;
arm_shoulder_y = torso_w / 2 - arm_r;

// Z coordinates
shoe_top  = shoe_h;
leg_top   = shoe_top + leg_h;
torso_top = leg_top + torso_h;

// ── Parts ─────────────────────────────────────────────────────────────────
module shoes() {
    color([0.10, 0.10, 0.12])
    for (y = [-shoe_gap, +shoe_gap])
        translate([0, y, shoe_h / 2])
            cube([shoe_len, shoe_w, shoe_h], center = true);
}

module legs() {
    color([0.20, 0.22, 0.30]) // dark pants
    for (y = [-shoe_gap, +shoe_gap])
        translate([0, y, shoe_top + leg_h / 2])
            cylinder(h = leg_h, r = leg_r, center = true);
}

module torso() {
    color([0.85, 0.30, 0.30]) // ski-jacket red
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
        difference() {
            sphere(r = head_r + 0.015);
            translate([0, 0, -head_r])
                cube([head_r * 3, head_r * 3, head_r * 2], center = true);
        }
}

module walker() {
    shoes();
    legs();
    torso();
    arms();
    head();
    helmet();
}

walker();

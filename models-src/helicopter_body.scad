// Helicopter body — static parts only. Main and tail rotors are separate
// animated parts declared via MOGUL_META part echoes below.
//
// Conventions: see helicopter.scad / models-src/README.md.

$fn = 10;

// ── Dimensions (keep in sync with helicopter_main_rotor.scad and
//    helicopter_tail_rotor.scad) ────────────────────────────────────────────

body_cx = 0.8;
body_l  = 4.5;
body_w  = 1.8;
body_h  = 2.0;
body_z  = 0.65;

nose_l  = 1.0;
nose_w  = 1.4;
nose_h  = 1.3;

boom_l    = 5.0;
boom_r1   = 0.38;
boom_r2   = 0.20;
boom_rise = 0.50;

fin_h   = 1.10;
fin_l   = 0.85;
fin_thk = 0.12;

stab_span = 1.8;
stab_l    = 0.65;
stab_thk  = 0.10;

skid_l = 4.0;
skid_r = 0.065;
skid_y = 0.82;
strut_r = 0.05;

mast_h = 0.50;
mast_r = 0.10;

// Derived
body_front = body_cx + body_l / 2;
body_rear  = body_cx - body_l / 2;
body_top   = body_z  + body_h;
boom_end_x = body_rear - boom_l;
boom_z_fwd = body_z + body_h * 0.38;
boom_z_end = boom_z_fwd + boom_rise;
mast_top   = body_top + mast_h;

// Tail rotor hub position (must match helicopter_tail_rotor.scad)
tr_x = boom_end_x + fin_l * 0.50;
tr_y = fin_thk / 2 + 0.06;
tr_z = boom_z_end + fin_h * 0.65;

// ── Animated part declarations ─────────────────────────────────────────────
// scad2obj reads these and emits "# part <name> <axis> <x> <y> <z>" in the
// OBJ header (game-space coordinates). The renderer loads <name>.obj and
// positions one instance per helicopter at body_pos + rotY(heading)*offset.
echo("MOGUL_META", "part", "helicopter_main_rotor", "spin_y", body_cx, 0, mast_top);
echo("MOGUL_META", "part", "helicopter_tail_rotor", "spin_z", tr_x, tr_y, tr_z);

// ── Modules ────────────────────────────────────────────────────────────────

module skids() {
    color([0.18, 0.18, 0.20])
    for (s = [-1, 1])
        translate([body_cx, s * skid_y, skid_r])
            rotate([0, 90, 0])
                cylinder(h = skid_l, r = skid_r, center = true);
}

module struts() {
    color([0.22, 0.22, 0.24])
    for (sx = [0.9, -0.9])
        for (s = [-1, 1])
            hull() {
                translate([body_cx + sx, s * body_w * 0.37, body_z])
                    sphere(r = strut_r);
                translate([body_cx + sx, s * skid_y, skid_r])
                    sphere(r = strut_r);
            }
}

module fuselage() {
    color([0.85, 0.12, 0.10])
    translate([body_cx, 0, body_z + body_h / 2])
        cube([body_l, body_w, body_h], center = true);
}

module nose() {
    color([0.85, 0.12, 0.10])
    hull() {
        translate([body_front, 0, body_z + body_h / 2])
            cube([0.02, body_w, body_h], center = true);
        translate([body_front + nose_l, 0, body_z + body_h / 2 - 0.25])
            cube([0.02, nose_w, nose_h], center = true);
    }
}

module windshield() {
    color([0.06, 0.10, 0.20])
    hull() {
        translate([body_front + 0.02, 0, body_z + body_h * 0.60])
            cube([0.02, body_w * 0.72, body_h * 0.36], center = true);
        translate([body_front + nose_l * 0.65, 0, body_z + body_h * 0.40])
            cube([0.02, nose_w * 0.70, nose_h * 0.45], center = true);
    }
}

module tail_boom() {
    color([0.78, 0.10, 0.08])
    hull() {
        translate([body_rear, 0, boom_z_fwd])
            sphere(r = boom_r1);
        translate([boom_end_x, 0, boom_z_end])
            sphere(r = boom_r2);
    }
}

module tail_fin() {
    fin_cx = boom_end_x + fin_l / 2;
    fin_cz = boom_z_end + fin_h / 2;
    color([0.72, 0.10, 0.08])
        translate([fin_cx, 0, fin_cz])
            cube([fin_l, fin_thk, fin_h], center = true);
}

module horizontal_stab() {
    stab_cx = boom_end_x + stab_l * 0.55;
    stab_cz = boom_z_end + 0.05;
    color([0.72, 0.10, 0.08])
        translate([stab_cx, 0, stab_cz])
            cube([stab_l, stab_span, stab_thk], center = true);
}

module rotor_mast() {
    color([0.28, 0.28, 0.30])
    translate([body_cx, 0, body_top])
        cylinder(h = mast_h, r = mast_r);
}

// ── Assembly ───────────────────────────────────────────────────────────────
skids();
struts();
fuselage();
nose();
windshield();
tail_boom();
tail_fin();
horizontal_stab();
rotor_mast();

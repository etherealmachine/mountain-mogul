// Helicopter — heli-ski transport. Ferries guests to off-piste drop zones.
// Based loosely on a Bell 407 / AS350 Écureuil class light helicopter.
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//   +X = forward (nose, direction of flight)
//   +Y = lateral
//   +Z = up
//
// Origin at Z = 0, centre of footprint (midpoint between skids at ground).
// Reference (Bell 407): fuselage 9.4 m · rotor diameter 10.7 m · height 3.6 m

$fn = 10;

// ── Dimensions (metres) ────────────────────────────────────────────────────

// Fuselage
body_cx = 0.8;   // X of block centre — shifted forward to give tail boom room
body_l  = 4.5;
body_w  = 1.8;
body_h  = 2.0;
body_z  = 0.65;  // Z of fuselage bottom (= strut height above skids)

// Nose / windscreen
nose_l  = 1.0;   // protrusion forward of fuselage front face
nose_w  = 1.4;   // tip width — slightly narrower than body
nose_h  = 1.3;   // tip height — shorter gives forward-leaning windscreen

// Tail boom
boom_l    = 5.0; // fuselage rear → tail rotor plane
boom_r1   = 0.38;
boom_r2   = 0.20;
boom_rise = 0.50; // Z gain from fuselage attach to tail end (up-sweep)

// Tail fin (vertical stabiliser at boom end)
fin_h   = 1.10;
fin_l   = 0.85;
fin_thk = 0.12;

// Horizontal stabilisers (small winglets at boom / fin junction)
stab_span = 1.8;
stab_l    = 0.65;
stab_thk  = 0.10;

// Landing skids
skid_l = 4.0;
skid_r = 0.065;
skid_y = 0.82;  // half-spacing (Y) between skids

// Struts — angled tubes from skids up to fuselage floor
strut_r = 0.05;

// Rotor mast
mast_h = 0.50;
mast_r = 0.10;

// Main rotor (3 blades)
blade_l = 5.0;   // hub centre to blade tip
blade_w = 0.30;  // chord (Y)
blade_h = 0.07;  // thickness (Z)
n_main  = 3;

// Tail rotor (2 blades; axis along Y, blades spin in XZ plane)
tr_l = 0.80;
tr_w = 0.18;  // chord (Z when blade is level)
tr_h = 0.05;  // thickness (Y)
n_tr = 2;

// ── Derived ────────────────────────────────────────────────────────────────
body_front = body_cx + body_l / 2;
body_rear  = body_cx - body_l / 2;
body_top   = body_z  + body_h;
boom_end_x = body_rear - boom_l;
boom_z_fwd = body_z + body_h * 0.38; // attachment Z at fuselage rear
boom_z_end = boom_z_fwd + boom_rise;  // tip Z at tail
mast_top   = body_top + mast_h;

// ── Modules ────────────────────────────────────────────────────────────────

module skids() {
    color([0.18, 0.18, 0.20])
    for (s = [-1, 1])
        translate([body_cx, s * skid_y, skid_r])
            rotate([0, 90, 0])
                cylinder(h = skid_l, r = skid_r, center = true);
}

module struts() {
    // hull() between two point-sized spheres traces an angled strut
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
    color([0.85, 0.12, 0.10]) // vivid red — classic heli-ski visibility colour
    translate([body_cx, 0, body_z + body_h / 2])
        cube([body_l, body_w, body_h], center = true);
}

module nose() {
    // hull between fuselage front face and a narrower, slightly lower slab
    // at the tip produces the angled windscreen profile
    color([0.85, 0.12, 0.10])
    hull() {
        translate([body_front, 0, body_z + body_h / 2])
            cube([0.02, body_w, body_h], center = true);
        translate([body_front + nose_l, 0, body_z + body_h / 2 - 0.25])
            cube([0.02, nose_w, nose_h], center = true);
    }
}

module windshield() {
    color([0.06, 0.10, 0.20]) // dark tinted glass
    hull() {
        translate([body_front + 0.02, 0, body_z + body_h * 0.60])
            cube([0.02, body_w * 0.72, body_h * 0.36], center = true);
        translate([body_front + nose_l * 0.65, 0, body_z + body_h * 0.40])
            cube([0.02, nose_w * 0.70, nose_h * 0.45], center = true);
    }
}

module tail_boom() {
    // Tapered tube via hull between two spheres of different radii; slight
    // Z rise gives the characteristic up-swept tail silhouette.
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

module main_rotor() {
    // Hub disc
    color([0.32, 0.32, 0.35])
    translate([body_cx, 0, mast_top])
        cylinder(h = 0.14, r = 0.24, center = true);

    // Blades — each starts at hub centre and extends outward in +X, then
    // rotated 120° apart around Z
    color([0.12, 0.12, 0.14])
    for (i = [0 : n_main - 1])
        translate([body_cx, 0, mast_top + 0.08])
            rotate([0, 0, i * 360 / n_main])
                translate([blade_l / 2, 0, 0])
                    cube([blade_l, blade_w, blade_h], center = true);
}

module tail_rotor() {
    // Hub axis along Y; blades fan in the XZ plane.
    // Mounted port-side (+Y) of the fin, near fin top.
    tr_x = boom_end_x + fin_l * 0.50;
    tr_y = fin_thk / 2 + 0.06;
    tr_z = boom_z_end + fin_h * 0.65;

    color([0.32, 0.32, 0.35])
    translate([tr_x, tr_y, tr_z])
        rotate([90, 0, 0])
            cylinder(h = 0.10, r = 0.12, center = true);

    // Two blades, 180° apart, extending along X from hub centre then rotated
    color([0.12, 0.12, 0.14])
    for (i = [0 : n_tr - 1])
        translate([tr_x, tr_y, tr_z])
            rotate([0, i * 360 / n_tr, 0])
                translate([tr_l / 2, 0, 0])
                    cube([tr_l, tr_h, tr_w], center = true);
}

// ── Assembly ───────────────────────────────────────────────────────────────
module helicopter() {
    skids();
    struts();
    fuselage();
    nose();
    windshield();
    tail_boom();
    tail_fin();
    horizontal_stab();
    rotor_mast();
    main_rotor();
    tail_rotor();
}

helicopter();

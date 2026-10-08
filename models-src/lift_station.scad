// Lift station — chairlift bottom / top terminal. A portal frame stands
// behind the load point and carries a boom; the bullwheel hangs from the
// boom under a shallow terminal hood, with an operator's hut beside the
// line.
//
// The bullwheel is centred on the origin at cable height (BullwheelHeight
// in internal/world/lift.go) with its rim at the cable gap (CableGap), so
// the up and down cables, which end at the origin 1.5 m either side, meet
// the wheel where they would wrap it. Chairs turn around at the origin and
// only run on the +X side, and the lift line queues along -X on the
// centreline (Lift.QueueSlotXZ), so the frame keeps its legs outside the
// chairs' sweep and nothing stands on the centreline at ground level.
//
// Sides: chairs carry riders up on -Y (the game's +Z after export) and
// come back empty on +Y, so the hut stands on +Y.
//
// Conventions, units, and axis docs: see models-src/README.md.

$fn = 16;

// ── Dimensions (metres) ────────────────────────────────────────────────
cable_h   = 3.65; // world.BullwheelHeight: cable height at the wheel
cable_gap = 1.50; // world.CableGap: up / down cable either side of the axis

// Bullwheel — vertical axis, horizontal disc, rim at the cable gap.
bw_r      = cable_gap + 0.05; // rim outer radius; the cable sits in its groove
bw_rim    = 0.28;             // radial depth of the rim
bw_thick  = 0.32;
hub_r     = 0.32;
spokes    = 6;

// Portal frame behind the wheel.
frame_x   = -2.2;  // legs and crossbeam, behind the wheel's rim
leg_y     = 3.6;   // outside the chairs (cable gap + about a chair's width)
leg_base  = 0.70;  // leg footprint at the ground
leg_top   = 0.40;  // and where it meets the crossbeam
frame_h   = 4.70;  // underside of the crossbeam
beam_h    = 0.45;  // crossbeam depth

// Boom from the crossbeam out over the wheel.
boom_w    = 0.70;
boom_h    = 0.40;
boom_z    = frame_h - 0.10 - boom_h; // just under the crossbeam

// Terminal hood over the wheel and boom.
hood_r    = 2.00;  // front radius, over the wheel
hood_back = -2.75; // back edge, past the crossbeam
hood_z    = frame_h;
hood_h    = 0.75;
hood_in   = 0.30;  // top edge inset, for a chamfered look

// Operator hut beside the empty side of the line.
hut_x     = -3.4;
hut_y     = 5.0;
hut_l     = 2.4;   // along X
hut_w     = 1.9;   // along Y
hut_h     = 2.4;

// ── Bullwheel ──────────────────────────────────────────────────────────
module bullwheel() {
    translate([0, 0, cable_h]) {
        // Rim: a ring whose outer edge carries the cable.
        color("Goldenrod")
            difference() {
                cylinder(h = bw_thick, r = bw_r, center = true, $fn = 32);
                cylinder(h = bw_thick + 0.02, r = bw_r - bw_rim, center = true, $fn = 32);
            }
        // Spokes and hub.
        color("DarkSlateGray") {
            for (i = [0 : spokes - 1])
                rotate([0, 0, i * 360 / spokes])
                    translate([(bw_r - bw_rim / 2) / 2, 0, 0])
                        cube([bw_r - bw_rim / 2, 0.14, 0.14], center = true);
            cylinder(h = 0.42, r = hub_r, center = true);
        }
    }
    // Shaft up to the boom.
    color("DimGray")
        translate([0, 0, cable_h + 0.2])
            cylinder(h = boom_z - cable_h - 0.2 + 0.01, r = 0.14);
}

// ── Portal frame and boom ──────────────────────────────────────────────
module leg(y) {
    color("SlateGray")
        hull() {
            translate([frame_x, y, 0.02])
                cube([leg_base, leg_base, 0.04], center = true);
            translate([frame_x, y, frame_h - 0.02])
                cube([leg_top, leg_top, 0.04], center = true);
        }
    // Concrete footing.
    color("Silver")
        translate([frame_x, y, 0.15])
            cube([leg_base + 0.35, leg_base + 0.35, 0.30], center = true);
}

module frame() {
    leg(leg_y);
    leg(-leg_y);
    color("SlateGray")
        translate([frame_x, 0, frame_h + beam_h / 2])
            cube([0.55, 2 * leg_y + leg_top, beam_h], center = true);
    // Boom: from behind the crossbeam out past the wheel's centre.
    color("LightSlateGray")
        translate([(frame_x - 0.4 + 0.5) / 2, 0, boom_z + boom_h / 2])
            cube([0.5 - (frame_x - 0.4), boom_w, boom_h], center = true);
    // Drive housing on the back of the crossbeam.
    color("DimGray")
        translate([frame_x - 0.9, 0, frame_h - 0.55])
            cube([1.0, 1.3, 1.0], center = true);
}

// ── Hood ───────────────────────────────────────────────────────────────
// A slab rounded over the wheel and square at the back, its top edge
// drawn in.
module hood_slab(inset, z) {
    translate([0, 0, z])
        hull() {
            cylinder(h = 0.02, r = hood_r - inset, $fn = 32);
            translate([hood_back + inset, -(hood_r - inset), 0])
                cube([0.02, 2 * (hood_r - inset), 0.02]);
        }
}

module hood() {
    color("Gainsboro")
        hull() {
            hood_slab(0, hood_z);
            hood_slab(hood_in, hood_z + hood_h);
        }
    // A coloured band round the hood's lower edge.
    color("SteelBlue")
        hull() {
            hood_slab(-0.02, hood_z - 0.01);
            hood_slab(-0.02, hood_z + 0.18);
        }
}

// ── Operator hut ───────────────────────────────────────────────────────
module hut() {
    translate([hut_x, hut_y, 0]) {
        // Walls.
        color("Sienna")
            translate([0, 0, hut_h / 2])
                cube([hut_l, hut_w, hut_h], center = true);
        // Windows facing the line (-Y) and the load point (+X).
        color("LightSteelBlue") {
            translate([0, -hut_w / 2 - 0.01, hut_h * 0.62])
                cube([hut_l * 0.7, 0.04, 0.8], center = true);
            translate([hut_l / 2 + 0.01, 0, hut_h * 0.62])
                cube([0.04, hut_w * 0.6, 0.8], center = true);
        }
        // Door on the back.
        color("SaddleBrown")
            translate([-hut_l / 2 - 0.01, 0.2, 0.95])
                cube([0.04, 0.8, 1.9], center = true);
        // Roof: a shallow pitch with an overhang.
        color("DarkSlateGray")
            hull() {
                translate([0, -0.15, hut_h + 0.05])
                    cube([hut_l + 0.5, hut_w + 0.5, 0.1], center = true);
                translate([0, 0.1, hut_h + 0.35])
                    cube([hut_l + 0.5, 0.2, 0.1], center = true);
            }
    }
}

module lift_station() {
    bullwheel();
    frame();
    hood();
    hut();
}

lift_station();

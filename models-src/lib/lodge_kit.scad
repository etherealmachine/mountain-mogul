// Lodge shell tile kit — shared dimensions and modules.
//
// The game paints a lodge footprint in 5 m cells and resolves the
// exterior from these tiles at half-cell (2.5 m) resolution; see
// internal/world/lodge_shell.go. Every tile is authored in the same
// canonical frame:
//
//   * origin on the lodge floor plane (Z = 0);
//   * wall-type tiles sit on a footprint edge, centred on it, with the
//     OUTSIDE toward SCAD −Y (game +Z) and the wall thickness toward +Y;
//   * roof tiles are centred on a half-cell, with Z = 0 at the tile's
//     lowest corner; slopes fall toward −Y (game +Z);
//   * corner tiles sit on a grid vertex; an outer corner's building is
//     in the SCAD (−X, +Y) quadrant, an inner corner's notch in (+X, −Y).
//
// Colours are near-neutral: the renderer tints walls and roofs per lodge
// from a seeded palette (world.ShellPalette).
//
// Keep these in sync with the constants in lodge_shell.go.

tile   = 2.5;   // ShellTileSize
wall_h = 5.0;   // ShellWallHeight
rise   = 2.1;   // ShellRoofRise, per tile of run
pitch  = rise / tile;

wall_t   = 0.35;  // wall thickness
sink     = 1.5;   // walls run this far below the floor to hide pad grading
roof_t   = 0.25;  // roof slab thickness
overhang = 0.7;   // eaves overhang past the wall plane

c_wall   = [0.92, 0.90, 0.86];
c_stone  = [0.62, 0.61, 0.60];
c_trim   = [0.97, 0.97, 0.95];
c_glass  = [0.30, 0.42, 0.52];
c_door   = [0.36, 0.25, 0.18];
c_roof   = [0.97, 0.97, 0.97];
c_fascia = [0.55, 0.52, 0.50];

// Plain wall panel: timber above a stone plinth, a trim band under the
// eaves.
module wall_panel() {
    color(c_wall)
        translate([-tile/2, 0, -sink]) cube([tile, wall_t, wall_h + sink]);
    color(c_stone)
        translate([-tile/2, -0.06, -sink]) cube([tile, wall_t, sink + 0.7]);
    color(c_trim)
        translate([-tile/2, -0.04, wall_h - 0.3]) cube([tile, 0.1, 0.3]);
    battens();
}

// Horizontal board lines above the plinth.
module battens() {
    color(c_wall * 0.8)
        for (z = [1.3 : 0.6 : wall_h - 0.6])
            translate([-tile/2, -0.03, z]) cube([tile, 0.04, 0.06]);
}

// A framed pane on the outer face. x0/w along the wall, z0/h up it.
module pane(x0, w, z0, h) {
    color(c_trim)
        translate([x0 - 0.1, -0.05, z0 - 0.1]) cube([w + 0.2, 0.06, h + 0.2]);
    color(c_glass)
        translate([x0, -0.08, z0]) cube([w, 0.06, h]);
}

module wall_window() {
    wall_panel();
    pane(-0.7, 1.4, 1.4, 2.2);
    color(c_trim) translate([-0.02, -0.1, 1.4]) cube([0.04, 0.04, 2.2]); // mullion
}

// Tall window flanked by shutters.
module wall_window_alt() {
    wall_panel();
    pane(-0.5, 1.0, 1.2, 2.8);
    color(c_door) {
        translate([-1.0, -0.1, 1.2]) cube([0.4, 0.05, 2.8]);
        translate([ 0.6, -0.1, 1.2]) cube([0.4, 0.05, 2.8]);
    }
}

// Floor-to-eave glazing for food courts: a low stone sill, then glass in
// three lights.
module wall_glazed() {
    color(c_wall)
        translate([-tile/2, 0, -sink]) cube([tile, wall_t, wall_h + sink]);
    color(c_stone)
        translate([-tile/2, -0.06, -sink]) cube([tile, wall_t, sink + 0.35]);
    color(c_glass)
        translate([-tile/2, -0.09, 0.35]) cube([tile, 0.06, wall_h - 0.75]);
    color(c_trim) {
        for (x = [-tile/2, -tile/6, tile/6, tile/2 - 0.08])
            translate([x, -0.12, 0.35]) cube([0.08, 0.06, wall_h - 0.75]);
        translate([-tile/2, -0.12, wall_h - 0.45]) cube([tile, 0.07, 0.1]);
    }
}

// Glazed entry door under a small canopy, with a step.
module door() {
    color(c_wall)
        translate([-tile/2, 0, -sink]) cube([tile, wall_t, wall_h + sink]);
    color(c_stone)
        translate([-tile/2, -0.06, -sink]) cube([tile, wall_t, sink + 0.15]);
    battens();
    color(c_door)
        translate([-0.9, -0.1, 0]) cube([1.8, 0.1, 2.7]);
    color(c_glass) {
        translate([-0.75, -0.13, 0.9]) cube([0.65, 0.05, 1.6]);
        translate([ 0.10, -0.13, 0.9]) cube([0.65, 0.05, 1.6]);
    }
    color(c_trim)
        translate([-tile/2, -0.04, wall_h - 0.3]) cube([tile, 0.1, 0.3]);
    color(c_roof)   // canopy
        translate([-1.2, -1.3, 3.0])
            rotate([-12, 0, 0]) cube([2.4, 1.3, 0.12]);
    color(c_stone)  // step
        translate([-1.1, -0.9, -0.3]) cube([2.2, 0.9, 0.42]);
}

module corner_outer() {
    color(c_stone)
        translate([-0.3, -0.05, -sink]) cube([0.35, 0.35, wall_h + sink - 0.3]);
}

module corner_inner() {
    color(c_trim)
        translate([0, -0.12, 0]) cube([0.12, 0.12, wall_h - 0.3]);
}

// Roof slab over one half-cell. h lists corner heights (in rise units)
// in SCAD order (−X,+Y), (+X,+Y), (+X,−Y), (−X,−Y); split names which
// diagonal the top surface folds along: 0 through corners 0–2, 1
// through corners 1–3.
module roof_tile(h, split) {
    p = [[-tile/2, tile/2], [tile/2, tile/2], [tile/2, -tile/2], [-tile/2, -tile/2]];
    top = [for (i = [0:3]) [p[i][0], p[i][1], h[i] * rise]];
    bot = [for (i = [0:3]) [p[i][0], p[i][1], h[i] * rise - roof_t]];
    tris = split == 0 ? [[0, 1, 2], [0, 2, 3]] : [[0, 1, 3], [1, 2, 3]];
    color(c_roof)
        polyhedron(
            points = concat(top, bot),
            faces = concat(
                // top (clockwise seen from above = outward in OpenSCAD)
                [for (t = tris) [t[0], t[1], t[2]]],
                [for (t = tris) [t[2] + 4, t[1] + 4, t[0] + 4]],
                [for (i = [0:3]) [i, i + 4, (i + 1) % 4 + 4, (i + 1) % 4]]
            ));
}

// Eaves strip past a wall edge: continues the roof plane outward and
// down, finished with a fascia board.
module eave() {
    drop = overhang * pitch;
    color(c_roof)
        polyhedron(
            points = [
                [-tile/2, 0, 0], [tile/2, 0, 0], [tile/2, -overhang, -drop], [-tile/2, -overhang, -drop],
                [-tile/2, 0, -roof_t], [tile/2, 0, -roof_t], [tile/2, -overhang, -drop - roof_t], [-tile/2, -overhang, -drop - roof_t],
            ],
            faces = [[0, 1, 2, 3], [7, 6, 5, 4], [0, 4, 5, 1], [1, 5, 6, 2], [2, 6, 7, 3], [3, 7, 4, 0]]);
    color(c_fascia)
        translate([-tile/2, -overhang - 0.05, -drop - roof_t - 0.1]) cube([tile, 0.08, roof_t + 0.2]);
}

// Eaves in the square past an outer corner: a hip folding along the
// diagonal away from the building.
module eave_corner() {
    o = overhang;
    d = o * pitch;
    color(c_roof)
        polyhedron(
            points = [
                [0, 0, 0], [o, 0, -d], [o, -o, -d], [0, -o, -d],
                [0, 0, -roof_t], [o, 0, -d - roof_t], [o, -o, -d - roof_t], [0, -o, -d - roof_t],
            ],
            faces = [[0, 1, 2], [0, 2, 3], [6, 5, 4], [7, 6, 4], [0, 4, 5, 1], [1, 5, 6, 2], [2, 6, 7, 3], [3, 7, 4, 0]]);
    color(c_fascia) {
        translate([0, -o - 0.05, -d - roof_t - 0.1]) cube([o + 0.05, 0.08, roof_t + 0.2]);
        translate([o - 0.03, -o - 0.05, -d - roof_t - 0.1]) cube([0.08, o + 0.05, roof_t + 0.2]);
    }
}

module chimney() {
    color(c_stone)
        translate([-0.55, -0.55, -1.0]) cube([1.1, 1.1, rise + 3.0]);
    color(c_fascia)
        translate([-0.65, -0.65, rise + 1.8]) cube([1.3, 1.3, 0.2]);
}

"""Lodge shell tile kit — shared dimensions and parts.

The game paints a lodge footprint in 5 m cells and resolves the exterior
from these tiles at half-cell (2.5 m) resolution; see
internal/world/lodge_shell.go. Every tile is authored in the same
canonical frame:

  * origin on the lodge floor plane (Z = 0);
  * wall-type tiles sit on a footprint edge, centred on it, with the
    OUTSIDE toward −Y (game +Z) and the wall thickness toward +Y;
  * roof tiles are centred on a half-cell, with Z = 0 at the tile's
    lowest corner; slopes fall toward −Y (game +Z);
  * corner tiles sit on a grid vertex; an outer corner's building is in
    the (−X, +Y) quadrant, an inner corner's notch in (+X, −Y).

Colours are near-neutral: the renderer tints walls and roofs per lodge
from a seeded palette (world.ShellPalette).

Keep these in sync with the constants in lodge_shell.go.

kind picks the building: "lodge" (timber and stone), "tent" (fabric on an
aluminium frame), or "shed" (corrugated metal). A tile file builds a
LodgeKit for its kind and calls one of the tile methods with its Model:

    from lib.lodge_kit import LodgeKit
    from lib.mogul import Model

    m = Model()
    LodgeKit("shed").wall_panel(m)
    m.main()
"""

from build123d import Face, Shell, Solid, Vector, Wire

from lib.mogul import cube, cylinder, rotate, translate

tile = 2.5  # ShellTileSize
sink = 1.5  # walls run this far below the floor to hide pad grading

c_wall = [0.92, 0.90, 0.86]
c_stone = [0.62, 0.61, 0.60]
c_trim = [0.97, 0.97, 0.95]
c_glass = [0.30, 0.42, 0.52]
c_door = [0.36, 0.25, 0.18]
c_roof = [0.97, 0.97, 0.97]
c_fascia = [0.55, 0.52, 0.50]
c_metal = [0.80, 0.80, 0.80]  # aluminium frame, shed trim
c_conc = [0.66, 0.65, 0.63]  # shed sill
c_vinyl = [0.62, 0.72, 0.78]  # tent window


def shade(c, k):
    """A colour scaled by k (OpenSCAD's c * k on a colour vector)."""
    return [x * k for x in c]


def box(pos, size):
    """cube(size) moved to pos (SCAD translate(pos) cube(size))."""
    return translate(pos, cube(size))


def frange(start, step, end):
    """OpenSCAD's [start : step : end], end inclusive."""
    out, i = [], 0
    while start + i * step <= end + 1e-9:
        out.append(start + i * step)
        i += 1
    return out


def slab(points, faces):
    """A polyhedron whose faces are wound as OpenSCAD wants them
    (clockwise seen from outside). Unlike mogul.polyhedron this keeps the
    given winding instead of orienting faces from the centroid, which
    gets non-convex shapes (the valley roof tile) wrong."""
    pts = [Vector(*p) for p in points]
    fs = [Face(Wire.make_polygon([pts[i] for i in reversed(f)], close=True)) for f in faces]
    s = Solid(Shell(fs))
    if s.volume < 0:
        s = Solid(Shell([f.reversed() for f in fs]))
    return s


class LodgeKit:
    """The shell tiles for one kind of building. Each tile method adds its
    coloured parts to a Model."""

    def __init__(self, kind="lodge"):
        assert kind in ("lodge", "shed", "tent")
        self.kind = kind
        self.wall_h = 4.0 if kind == "shed" else 3.0 if kind == "tent" else 5.0  # ShellKind.WallHeight
        self.rise = 0.8 if kind == "shed" else 2.6 if kind == "tent" else 2.1  # ShellKind.RoofRise, per tile of run
        self.pitch = self.rise / tile
        self.wall_t = 0.35 if kind == "lodge" else 0.15 if kind == "shed" else 0.08  # wall thickness
        self.roof_t = 0.25 if kind == "lodge" else 0.12 if kind == "shed" else 0.06  # roof slab thickness
        self.overhang = 0.7 if kind == "lodge" else 0.35 if kind == "shed" else 0.15  # eaves overhang past the wall plane

    # ── Dispatch by kind. Tile files call these. ───────────────────────

    def wall_panel(self, m):
        {"shed": lambda: self.shed_wall(m, 0), "tent": lambda: self.tent_wall(m, 0)}.get(
            self.kind, lambda: self.lodge_wall_panel(m)
        )()

    def wall_window(self, m):
        {"shed": lambda: self.shed_wall(m, 1), "tent": lambda: self.tent_wall(m, 1)}.get(
            self.kind, lambda: self.lodge_wall_window(m)
        )()

    def wall_window_alt(self, m):
        {"shed": lambda: self.shed_wall(m, 2), "tent": lambda: self.tent_wall(m, 2)}.get(
            self.kind, lambda: self.lodge_wall_window_alt(m)
        )()

    def wall_glazed(self, m):
        {"shed": lambda: self.shed_wall(m, 3), "tent": lambda: self.tent_wall(m, 3)}.get(
            self.kind, lambda: self.lodge_wall_glazed(m)
        )()

    def door(self, m):
        {"shed": lambda: self.shed_door(m), "tent": lambda: self.tent_door(m)}.get(
            self.kind, lambda: self.lodge_door(m)
        )()

    def corner_outer(self, m):
        if self.kind == "lodge":
            self.lodge_corner_outer(m)
        else:
            self.post(m, 0.16, self.wall_h + sink, -sink)

    def corner_inner(self, m):
        if self.kind == "lodge":
            self.lodge_corner_inner(m)
        else:
            self.post(m, 0.08, self.wall_h, 0)

    def chimney(self, m):
        if self.kind == "lodge":
            self.lodge_chimney(m)
        else:
            self.stove_pipe(m)

    # ── Lodge: timber above a stone plinth ─────────────────────────────

    def lodge_wall_panel(self, m):
        """Plain wall panel: timber above a stone plinth, a trim band
        under the eaves."""
        wh, wt = self.wall_h, self.wall_t
        m.add(box([-tile / 2, 0, -sink], [tile, wt, wh + sink]), c_wall)
        m.add(box([-tile / 2, -0.06, -sink], [tile, wt, sink + 0.7]), c_stone)
        m.add(box([-tile / 2, -0.04, wh - 0.3], [tile, 0.1, 0.3]), c_trim)
        self.battens(m)

    def battens(self, m):
        """Horizontal board lines above the plinth."""
        m.add(
            [box([-tile / 2, -0.03, z], [tile, 0.04, 0.06]) for z in frange(1.3, 0.6, self.wall_h - 0.6)],
            shade(c_wall, 0.8),
        )

    def pane(self, m, x0, w, z0, h):
        """A framed pane on the outer face. x0/w along the wall, z0/h up
        it."""
        m.add(box([x0 - 0.1, -0.05, z0 - 0.1], [w + 0.2, 0.06, h + 0.2]), c_trim)
        m.add(box([x0, -0.08, z0], [w, 0.06, h]), c_glass)

    def lodge_wall_window(self, m):
        self.lodge_wall_panel(m)
        self.pane(m, -0.7, 1.4, 1.4, 2.2)
        m.add(box([-0.02, -0.1, 1.4], [0.04, 0.04, 2.2]), c_trim)  # mullion

    def lodge_wall_window_alt(self, m):
        """Tall window flanked by shutters."""
        self.lodge_wall_panel(m)
        self.pane(m, -0.5, 1.0, 1.2, 2.8)
        m.add(
            [box([-1.0, -0.1, 1.2], [0.4, 0.05, 2.8]), box([0.6, -0.1, 1.2], [0.4, 0.05, 2.8])],
            c_door,
        )

    def lodge_wall_glazed(self, m):
        """Floor-to-eave glazing for food courts: a low stone sill, then
        glass in three lights."""
        wh, wt = self.wall_h, self.wall_t
        m.add(box([-tile / 2, 0, -sink], [tile, wt, wh + sink]), c_wall)
        m.add(box([-tile / 2, -0.06, -sink], [tile, wt, sink + 0.35]), c_stone)
        m.add(box([-tile / 2, -0.09, 0.35], [tile, 0.06, wh - 0.75]), c_glass)
        m.add(
            [box([x, -0.12, 0.35], [0.08, 0.06, wh - 0.75]) for x in (-tile / 2, -tile / 6, tile / 6, tile / 2 - 0.08)]
            + [box([-tile / 2, -0.12, wh - 0.45], [tile, 0.07, 0.1])],
            c_trim,
        )

    def lodge_door(self, m):
        """Glazed entry door under a small canopy, with a step."""
        wh, wt = self.wall_h, self.wall_t
        m.add(box([-tile / 2, 0, -sink], [tile, wt, wh + sink]), c_wall)
        m.add(box([-tile / 2, -0.06, -sink], [tile, wt, sink + 0.15]), c_stone)
        self.battens(m)
        m.add(box([-0.9, -0.1, 0], [1.8, 0.1, 2.7]), c_door)
        m.add([box([-0.75, -0.13, 0.9], [0.65, 0.05, 1.6]), box([0.10, -0.13, 0.9], [0.65, 0.05, 1.6])], c_glass)
        m.add(box([-tile / 2, -0.04, wh - 0.3], [tile, 0.1, 0.3]), c_trim)
        m.add(translate([-1.2, -1.3, 3.0], rotate([-12, 0, 0], cube([2.4, 1.3, 0.12]))), c_roof)  # canopy
        m.add(box([-1.1, -0.9, -0.3], [2.2, 0.9, 0.42]), c_stone)  # step

    def lodge_corner_outer(self, m):
        m.add(box([-0.3, -0.05, -sink], [0.35, 0.35, self.wall_h + sink - 0.3]), c_stone)

    def lodge_corner_inner(self, m):
        m.add(box([0, -0.12, 0], [0.12, 0.12, self.wall_h - 0.3]), c_trim)

    def lodge_chimney(self, m):
        m.add(box([-0.55, -0.55, -1.0], [1.1, 1.1, self.rise + 3.0]), c_stone)
        m.add(box([-0.65, -0.65, self.rise + 1.8], [1.3, 1.3, 0.2]), c_fascia)

    # ── Roofs and eaves (every kind) ───────────────────────────────────

    def roof_tile(self, m, h, split):
        """Roof slab over one half-cell. h lists corner heights (in rise
        units) in order (−X,+Y), (+X,+Y), (+X,−Y), (−X,−Y); split names
        which diagonal the top surface folds along: 0 through corners 0–2,
        1 through corners 1–3."""
        p = [(-tile / 2, tile / 2), (tile / 2, tile / 2), (tile / 2, -tile / 2), (-tile / 2, -tile / 2)]
        top = [(p[i][0], p[i][1], h[i] * self.rise) for i in range(4)]
        bot = [(p[i][0], p[i][1], h[i] * self.rise - self.roof_t) for i in range(4)]
        tris = [[0, 1, 2], [0, 2, 3]] if split == 0 else [[0, 1, 3], [1, 2, 3]]
        faces = (
            # top (clockwise seen from above = outward, OpenSCAD winding)
            [[t[0], t[1], t[2]] for t in tris]
            + [[t[2] + 4, t[1] + 4, t[0] + 4] for t in tris]
            + [[i, i + 4, (i + 1) % 4 + 4, (i + 1) % 4] for i in range(4)]
        )
        m.add(slab(top + bot, faces), c_roof)

    def eave(self, m):
        """Eaves strip past a wall edge: continues the roof plane outward
        and down, finished with a fascia board."""
        o, t = self.overhang, self.roof_t
        drop = o * self.pitch
        m.add(
            slab(
                [
                    (-tile / 2, 0, 0), (tile / 2, 0, 0), (tile / 2, -o, -drop), (-tile / 2, -o, -drop),
                    (-tile / 2, 0, -t), (tile / 2, 0, -t), (tile / 2, -o, -drop - t), (-tile / 2, -o, -drop - t),
                ],
                [[0, 1, 2, 3], [7, 6, 5, 4], [0, 4, 5, 1], [1, 5, 6, 2], [2, 6, 7, 3], [3, 7, 4, 0]],
            ),
            c_roof,
        )
        m.add(box([-tile / 2, -o - 0.05, -drop - t - 0.1], [tile, 0.08, t + 0.2]), c_fascia)

    def eave_corner(self, m):
        """Eaves in the square past an outer corner: a hip folding along
        the diagonal away from the building."""
        o, t = self.overhang, self.roof_t
        d = o * self.pitch
        m.add(
            slab(
                [
                    (0, 0, 0), (o, 0, -d), (o, -o, -d), (0, -o, -d),
                    (0, 0, -t), (o, 0, -d - t), (o, -o, -d - t), (0, -o, -d - t),
                ],
                [[0, 1, 2], [0, 2, 3], [6, 5, 4], [7, 6, 4], [0, 4, 5, 1], [1, 5, 6, 2], [2, 6, 7, 3], [3, 7, 4, 0]],
            ),
            c_roof,
        )
        m.add(
            [
                box([0, -o - 0.05, -d - t - 0.1], [o + 0.05, 0.08, t + 0.2]),
                box([o - 0.03, -o - 0.05, -d - t - 0.1], [0.08, o + 0.05, t + 0.2]),
            ],
            c_fascia,
        )

    # ── Shed: corrugated metal on a concrete sill ──────────────────────
    # v: 0 plain, 1 a high window, 2 a louvred vent, 3 a translucent band.

    def shed_sheet(self, m, z0, z1):
        m.add(box([-tile / 2, 0, z0], [tile, self.wall_t, z1 - z0]), c_wall)
        m.add(
            [box([x, -0.04, z0], [0.06, 0.05, z1 - z0]) for x in frange(-tile / 2 + 0.08, 0.25, tile / 2 - 0.05)],
            shade(c_wall, 0.82),
        )

    def shed_wall(self, m, v):
        wh, wt = self.wall_h, self.wall_t
        self.shed_sheet(m, -sink + 0.4, wh)
        m.add(box([-tile / 2, -0.03, -sink], [tile, wt + 0.03, sink + 0.4]), c_conc)
        m.add(box([-tile / 2, -0.07, wh - 0.18], [tile, 0.08, 0.18]), c_metal)
        if v == 1:
            m.add(box([-0.7, -0.09, wh - 1.3], [1.4, 0.06, 0.8]), c_metal)
            m.add(box([-0.6, -0.11, wh - 1.2], [1.2, 0.04, 0.6]), c_glass)
        if v == 2:
            m.add(
                [translate([-0.5, -0.12, z], rotate([-30, 0, 0], cube([1.0, 0.03, 0.14]))) for z in frange(1.6, 0.18, 2.4)],
                shade(c_metal, 0.7),
            )
        if v == 3:
            m.add(box([-tile / 2, -0.09, 1.2], [tile, 0.05, 1.0]), c_vinyl)

    def shed_door(self, m):
        """A roll-up door with slats, under a box housing."""
        wh, wt = self.wall_h, self.wall_t
        m.add(box([-tile / 2, -0.03, -sink], [tile, wt + 0.03, sink + 0.1]), c_conc)
        m.add(
            [
                box([-tile / 2, 0, 0.1], [0.15, wt, wh - 0.1]),
                box([tile / 2 - 0.15, 0, 0.1], [0.15, wt, wh - 0.1]),
                box([-tile / 2, 0, 3.1], [tile, wt, wh - 3.1]),
            ],
            c_wall,
        )
        m.add(box([-tile / 2 + 0.15, 0.02, 0], [tile - 0.3, 0.05, 3.1]), shade(c_door, 1.4))
        m.add([box([-tile / 2 + 0.15, -0.01, z], [tile - 0.3, 0.04, 0.05]) for z in frange(0.2, 0.3, 3.0)], c_door)
        m.add(box([-tile / 2, -0.25, 3.1], [tile, 0.25, 0.35]), c_metal)

    # ── Tent: fabric panels between aluminium posts ────────────────────
    # v: 0 plain, 1 a clear window, 2 two small windows, 3 a wide clear panel.

    def tent_wall(self, m, v):
        wh, wt = self.wall_h, self.wall_t
        m.add(box([-tile / 2, 0, -0.4], [tile, wt, wh + 0.4]), c_wall)
        m.add(
            [
                box([-0.02, -0.02, -0.4], [0.04, 0.03, wh + 0.4]),  # seam
                box([-tile / 2, -0.03, -0.4], [tile, 0.05, 0.5]),  # skirt
            ],
            shade(c_wall, 0.88),
        )
        m.add(box([-tile / 2, -0.05, wh - 0.1], [tile, 0.07, 0.1]), c_metal)  # eave rail
        if v == 1:
            m.add(box([-0.8, -0.04, 1.0], [1.6, 0.03, 1.3]), c_vinyl)
        if v == 2:
            m.add([box([-1.0, -0.04, 1.2], [0.7, 0.03, 1.0]), box([0.3, -0.04, 1.2], [0.7, 0.03, 1.0])], c_vinyl)
        if v == 3:
            m.add(box([-tile / 2 + 0.1, -0.04, 0.4], [tile - 0.2, 0.03, wh - 0.8]), c_vinyl)

    def tent_door(self, m):
        """An open doorway with its flaps tied back."""
        wh, wt = self.wall_h, self.wall_t
        m.add(
            [
                box([-tile / 2, 0, -0.4], [0.35, wt, wh + 0.4]),
                box([tile / 2 - 0.35, 0, -0.4], [0.35, wt, wh + 0.4]),
                box([-tile / 2, 0, 2.3], [tile, wt, wh - 2.3]),
            ],
            c_wall,
        )
        m.add(box([-tile / 2 + 0.35, 0.3, 0], [tile - 0.7, 0.05, 2.3]), shade(c_door, 0.6))  # the dim inside
        m.add(
            [
                box([-tile / 2 + 0.3, -0.1, 0], [0.25, 0.12, 2.3]),  # rolled flaps
                box([tile / 2 - 0.55, -0.1, 0], [0.25, 0.12, 2.3]),
            ],
            shade(c_wall, 0.9),
        )
        m.add(box([-tile / 2, -0.05, wh - 0.1], [tile, 0.07, 0.1]), c_metal)

    # ── Tent and shed corners and chimneys ─────────────────────────────

    def post(self, m, w, h, z0):
        """A square post at a corner, for tents and sheds."""
        m.add(box([-w / 2, -w / 2, z0], [w, w, h]), c_metal)

    def stove_pipe(self, m):
        """A stove pipe in place of a chimney (tents and sheds don't use
        one)."""
        m.add(translate([0, 0, -0.5], cylinder(h=self.rise + 1.5, r=0.15, fn=12)), c_metal)

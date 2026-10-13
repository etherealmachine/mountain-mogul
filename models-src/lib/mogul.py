"""Mountain Mogul model kit: build123d helpers and the OBJ writer.

Every model in models-src/ is a Python script that builds its parts with
build123d (directly, or through the SCAD-style helpers below), adds each
part to a Model with its colour, and calls Model.main(), which writes the
OBJ named on the command line:

    from lib.mogul import *

    m = Model()
    m.add(cylinder(h=1.3, r=0.18, fn=6), "SaddleBrown")
    m.main()

Authoring space is Z-up, metres, as the OpenSCAD sources were: +X is the
model's primary direction, +Y across, +Z up. The writer turns it into the
game's Y-up space, (x, y, z) -> (x, z, -y), and writes the OBJ the game's
loader reads: per-vertex colour as `v x y z r g b`, one flat normal per
triangle, and metadata comments at the head (`# slot`, `# footprint`,
`# part`; see models-src/README.md).

Curves: build123d makes true curves, tessellated to the Model's tolerance
(or a part's own). Models the game draws by the thousand (trees, chairs,
cars, wall tiles) keep their triangle counts low by asking for faceted
primitives with fn=, as OpenSCAD's $fn did.
"""

import math
import os
import sys

import numpy as np
from build123d import (
    Axis,
    Box,
    Compound,
    Cone,
    Cylinder,
    Face,
    Location,
    Plane,
    Polyline,
    Pos,
    Rot,
    Shell,
    Solid,
    Sphere,
    Vector,
    Wire,
    extrude,
    loft,
    make_face,
    mirror as _mirror,
)
from scipy.spatial import ConvexHull as _ConvexHull

__all__ = [
    "Model",
    "cube",
    "cylinder",
    "sphere",
    "prism",
    "translate",
    "rotate",
    "mirror",
    "hull",
    "union",
    "difference",
    "linear_extrude",
    "polyhedron",
    "tube",
    "rod",
    "rounded_rect",
    "gable",
    "hull_points",
    "Vector",
    "math",
]

# ── Colours ────────────────────────────────────────────────────────────
# The CSS / SVG named colours OpenSCAD's color() accepts.
_CSS = {
    "aliceblue": "f0f8ff", "antiquewhite": "faebd7", "aqua": "00ffff", "aquamarine": "7fffd4",
    "azure": "f0ffff", "beige": "f5f5dc", "bisque": "ffe4c4", "black": "000000",
    "blanchedalmond": "ffebcd", "blue": "0000ff", "blueviolet": "8a2be2", "brown": "a52a2a",
    "burlywood": "deb887", "cadetblue": "5f9ea0", "chartreuse": "7fff00", "chocolate": "d2691e",
    "coral": "ff7f50", "cornflowerblue": "6495ed", "cornsilk": "fff8dc", "crimson": "dc143c",
    "cyan": "00ffff", "darkblue": "00008b", "darkcyan": "008b8b", "darkgoldenrod": "b8860b",
    "darkgray": "a9a9a9", "darkgreen": "006400", "darkgrey": "a9a9a9", "darkkhaki": "bdb76b",
    "darkmagenta": "8b008b", "darkolivegreen": "556b2f", "darkorange": "ff8c00", "darkorchid": "9932cc",
    "darkred": "8b0000", "darksalmon": "e9967a", "darkseagreen": "8fbc8f", "darkslateblue": "483d8b",
    "darkslategray": "2f4f4f", "darkslategrey": "2f4f4f", "darkturquoise": "00ced1", "darkviolet": "9400d3",
    "deeppink": "ff1493", "deepskyblue": "00bfff", "dimgray": "696969", "dimgrey": "696969",
    "dodgerblue": "1e90ff", "firebrick": "b22222", "floralwhite": "fffaf0", "forestgreen": "228b22",
    "fuchsia": "ff00ff", "gainsboro": "dcdcdc", "ghostwhite": "f8f8ff", "gold": "ffd700",
    "goldenrod": "daa520", "gray": "808080", "green": "008000", "greenyellow": "adff2f",
    "grey": "808080", "honeydew": "f0fff0", "hotpink": "ff69b4", "indianred": "cd5c5c",
    "indigo": "4b0082", "ivory": "fffff0", "khaki": "f0e68c", "lavender": "e6e6fa",
    "lavenderblush": "fff0f5", "lawngreen": "7cfc00", "lemonchiffon": "fffacd", "lightblue": "add8e6",
    "lightcoral": "f08080", "lightcyan": "e0ffff", "lightgoldenrodyellow": "fafad2", "lightgray": "d3d3d3",
    "lightgreen": "90ee90", "lightgrey": "d3d3d3", "lightpink": "ffb6c1", "lightsalmon": "ffa07a",
    "lightseagreen": "20b2aa", "lightskyblue": "87cefa", "lightslategray": "778899", "lightslategrey": "778899",
    "lightsteelblue": "b0c4de", "lightyellow": "ffffe0", "lime": "00ff00", "limegreen": "32cd32",
    "linen": "faf0e6", "magenta": "ff00ff", "maroon": "800000", "mediumaquamarine": "66cdaa",
    "mediumblue": "0000cd", "mediumorchid": "ba55d3", "mediumpurple": "9370db", "mediumseagreen": "3cb371",
    "mediumslateblue": "7b68ee", "mediumspringgreen": "00fa9a", "mediumturquoise": "48d1cc", "mediumvioletred": "c71585",
    "midnightblue": "191970", "mintcream": "f5fffa", "mistyrose": "ffe4e1", "moccasin": "ffe4b5",
    "navajowhite": "ffdead", "navy": "000080", "oldlace": "fdf5e6", "olive": "808000",
    "olivedrab": "6b8e23", "orange": "ffa500", "orangered": "ff4500", "orchid": "da70d6",
    "palegoldenrod": "eee8aa", "palegreen": "98fb98", "paleturquoise": "afeeee", "palevioletred": "db7093",
    "papayawhip": "ffefd5", "peachpuff": "ffdab9", "peru": "cd853f", "pink": "ffc0cb",
    "plum": "dda0dd", "powderblue": "b0e0e6", "purple": "800080", "rebeccapurple": "663399",
    "red": "ff0000", "rosybrown": "bc8f8f", "royalblue": "4169e1", "saddlebrown": "8b4513",
    "salmon": "fa8072", "sandybrown": "f4a460", "seagreen": "2e8b57", "seashell": "fff5ee",
    "sienna": "a0522d", "silver": "c0c0c0", "skyblue": "87ceeb", "slateblue": "6a5acd",
    "slategray": "708090", "slategrey": "708090", "snow": "fffafa", "springgreen": "00ff7f",
    "steelblue": "4682b4", "tan": "d2b48c", "teal": "008080", "thistle": "d8bfd8",
    "tomato": "ff6347", "turquoise": "40e0d0", "violet": "ee82ee", "wheat": "f5deb3",
    "white": "ffffff", "whitesmoke": "f5f5f5", "yellow": "ffff00", "yellowgreen": "9acd32",
}


def rgb(color):
    """An RGB triple in 0..1 from a CSS name, "#rrggbb", or an RGB(A) list."""
    if isinstance(color, str):
        h = color.lstrip("#") if color.startswith("#") else _CSS[color.lower()]
        return tuple(int(h[i : i + 2], 16) / 255 for i in (0, 2, 4))
    return tuple(float(c) for c in color[:3])


# ── SCAD-style primitives ──────────────────────────────────────────────
# Same placement rules as OpenSCAD: cube and cylinder sit on the origin
# corner / base unless center=True.


def _vec3(v):
    if isinstance(v, (int, float)):
        return (float(v),) * 3
    return tuple(float(c) for c in v)


def cube(size, center=False):
    """A box; at the origin corner, or centred."""
    x, y, z = _vec3(size)
    b = Box(x, y, z)
    return b if center else b.moved(Location((x / 2, y / 2, z / 2)))


def prism(r1, r2, h, n):
    """A regular n-sided prism (r1 == r2) or frustum, base on z = 0,
    radii to the corners, a corner on +X like OpenSCAD's $fn shapes."""

    def ring(r, z):
        pts = [(r * math.cos(2 * math.pi * i / n), r * math.sin(2 * math.pi * i / n), z) for i in range(n)]
        return pts

    if r2 <= 1e-6:
        return polyhedron(
            ring(r1, 0) + [(0, 0, h)],
            [list(reversed(range(n)))] + [[i, (i + 1) % n, n] for i in range(n)],
        )
    if r1 <= 1e-6:
        return polyhedron(
            [(0, 0, 0)] + ring(r2, h),
            [list(range(1, n + 1))] + [[0, 1 + (i + 1) % n, 1 + i] for i in range(n)],
        )
    if abs(r1 - r2) < 1e-9:
        face = make_face(Polyline(*[(x, y) for x, y, _ in ring(r1, 0)], close=True))
        return extrude(face, h)
    bottom, top = ring(r1, 0), ring(r2, h)
    faces = [list(reversed(range(n))), [n + i for i in range(n)]]
    faces += [[i, (i + 1) % n, n + (i + 1) % n, n + i] for i in range(n)]
    return polyhedron(bottom + top, faces)


def cylinder(h, r=None, r1=None, r2=None, d=None, d1=None, d2=None, center=False, fn=None):
    """A cylinder or cone along +Z, base on z = 0 unless center. fn gives
    a faceted one with that many sides (OpenSCAD's $fn); without it a true
    curve."""
    if d is not None:
        r = d / 2
    if d1 is not None:
        r1 = d1 / 2
    if d2 is not None:
        r2 = d2 / 2
    if r is not None:
        r1 = r if r1 is None else r1
        r2 = r if r2 is None else r2
    r1 = 1.0 if r1 is None else r1
    r2 = r1 if r2 is None else r2
    if fn:
        s = prism(r1, r2, h, fn)
    elif abs(r1 - r2) < 1e-9:
        s = Cylinder(r1, h).moved(Location((0, 0, h / 2)))
    else:
        s = Cone(r1, r2, h).moved(Location((0, 0, h / 2)))
    return s.moved(Location((0, 0, -h / 2))) if center else s


def sphere(r=None, d=None, fn=None):
    """A sphere at the origin. fn gives a faceted one (a hull of rings)."""
    if d is not None:
        r = d / 2
    if not fn:
        return Sphere(r)
    rings = max(fn // 2, 2)
    pts = []
    for i in range(rings):
        phi = math.pi * (i + 0.5) / rings
        z, rr = r * math.cos(phi), r * math.sin(phi)
        for k in range(fn):
            a = 2 * math.pi * k / fn
            pts.append((rr * math.cos(a), rr * math.sin(a), z))
    return hull_points(np.array(pts))


def translate(v, shape):
    x, y, z = _vec3(v)
    return shape.moved(Location((x, y, z)))


def rotate(a, shape, v=None):
    """OpenSCAD rotate: rotate([ax, ay, az], s) turns about X, then Y,
    then Z (degrees, about the origin); rotate(a, s, v=axis) about an
    axis."""
    if v is not None:
        return shape.rotate(Axis((0, 0, 0), v), a)
    if isinstance(a, (int, float)):
        a = (0, 0, a)
    ax, ay, az = _vec3(a)
    s = shape
    if ax:
        s = s.rotate(Axis.X, ax)
    if ay:
        s = s.rotate(Axis.Y, ay)
    if az:
        s = s.rotate(Axis.Z, az)
    return s


def mirror(v, shape):
    """OpenSCAD mirror: reflect across the plane through the origin with
    normal v."""
    return _mirror(shape, Plane(origin=(0, 0, 0), z_dir=_vec3(v)))


def union(*shapes):
    shapes = [s for s in _flat(shapes) if s is not None]
    out = shapes[0]
    for s in shapes[1:]:
        out = out + s
    return out


def difference(shape, *cutters):
    out = shape
    for c in _flat(cutters):
        out = out - c
    return out


def _flat(items):
    for it in items:
        if isinstance(it, (list, tuple)):
            yield from _flat(it)
        else:
            yield it


# ── Hull ───────────────────────────────────────────────────────────────


def _points_of(shape, tol=0.002):
    verts, _ = shape.tessellate(tol, 0.1)
    pts = [(v.X, v.Y, v.Z) for v in verts]
    # Tessellation can skip a solid's own corners on curved edges; add its
    # vertices too.
    pts += [(v.X, v.Y, v.Z) for v in shape.vertices()]
    return pts


def hull_points(pts):
    """The convex hull of raw (x, y, z) points, as a solid."""
    pts = np.asarray(pts, dtype=float)
    h = _ConvexHull(pts)
    faces = []
    centre = pts.mean(axis=0)
    for simplex, eq in zip(h.simplices, h.equations):
        a, b, c = (pts[i] for i in simplex)
        n = np.cross(b - a, c - a)
        if np.dot(n, eq[:3]) < 0:
            simplex = simplex[[0, 2, 1]]
        faces.append([int(i) for i in simplex])
    return polyhedron([tuple(p) for p in pts], faces)


def hull(*shapes):
    """OpenSCAD hull(): the convex hull of the shapes, as a solid."""
    pts = []
    for s in _flat(shapes):
        pts += _points_of(s)
    return hull_points(np.array(pts))


# ── Extrusions and polyhedra ───────────────────────────────────────────


def linear_extrude(points, height, scale=1.0, center=False):
    """OpenSCAD linear_extrude of a polygon (2D points, counter-clockwise
    or not) up +Z, scaled about the origin to scale (a number or [sx, sy])
    at the top."""
    sx, sy = (scale, scale) if isinstance(scale, (int, float)) else scale
    z0 = -height / 2 if center else 0
    if sx == 1 and sy == 1:
        s = extrude(make_face(Polyline(*points, close=True)), height)
        return s.moved(Location((0, 0, z0)))
    bottom = [(x, y, z0) for x, y in points]
    top = [(x * sx, y * sy, z0 + height) for x, y in points]
    return hull_points(np.array(bottom + top))


def polyhedron(points, faces):
    """A solid from points and faces (lists of point indices), as
    OpenSCAD's: every face wound the same way round. The whole solid is
    turned inside out if the winding faces in."""
    pts = [Vector(*_vec3(p)) for p in points]
    rings = []
    for f in faces:
        ring = [pts[i] for i in f]
        # Drop repeated points (a degenerate edge would break the face).
        ring = [p for k, p in enumerate(ring) if (p - ring[k - 1]).length > 1e-9]
        if len(ring) >= 3:
            rings.append(ring)
    # Signed volume by the divergence theorem: negative when the faces'
    # winding points them inward.
    vol = 0.0
    for ring in rings:
        for k in range(1, len(ring) - 1):
            vol += ring[0].dot(ring[k].cross(ring[k + 1]))
    if vol < 0:
        rings = [list(reversed(r)) for r in rings]
    return Solid(Shell([Face(Wire.make_polygon(r, close=True)) for r in rings]))


# ── Profiles ───────────────────────────────────────────────────────────
# Cheap rounding: a rounded outline extruded, with a few flat facets per
# round, costs far fewer triangles than a build123d fillet.


def rounded_rect(a, b, r, seg=3):
    """A centred a × b rectangle with its corners rounded to r, as 2D points."""
    pts = []
    for cx, cy, a0 in ((a / 2 - r, -b / 2 + r, -90), (a / 2 - r, b / 2 - r, 0),
                       (-a / 2 + r, b / 2 - r, 90), (-a / 2 + r, -b / 2 + r, 180)):
        for k in range(seg + 1):
            t = math.radians(a0 + 90 * k / seg)
            pts.append((cx + r * math.cos(t), cy + r * math.sin(t)))
    return pts


def gable(length, span, rise, r, seg=4):
    """A gable roof: a triangle span wide and rise high, its ridge rounded
    to r, run along X for length (centred), its base on z = 0."""
    hb = span / 2
    n = math.hypot(rise, hb)
    nx, nz = rise / n, hb / n  # the right slope's outward normal
    cz = rise - r * n / hb  # the ridge arc's centre (r / sin of the half-angle)
    a0 = math.atan2(nz, nx)
    arc = [(r * math.cos(a), cz + r * math.sin(a))
           for a in (a0 + (math.pi - 2 * a0) * k / seg for k in range(seg + 1))]
    profile = [(-hb, 0), (hb, 0)] + arc
    # The profile is in YZ (its x is Y); extrude it along +X.
    return rotate([90, 0, 90], linear_extrude(profile, height=length, center=True))


# ── Members ────────────────────────────────────────────────────────────


def tube(a, b, s):
    """A square-section member s thick from point a to point b (the hull of
    two s-cubes, as the SCAD kits drew frame tubes)."""
    return hull(translate(a, cube(s, center=True)), translate(b, cube(s, center=True)))


def rod(a, b, r, fn=None):
    """A round member of radius r from point a to point b, square-ended."""
    a, b = Vector(*_vec3(a)), Vector(*_vec3(b))
    d = b - a
    length = d.length
    c = cylinder(h=length, r=r, fn=fn)
    return c.moved(Location(Plane(origin=a, z_dir=d.normalized())))


# ── The model and its OBJ ──────────────────────────────────────────────


class Model:
    """The coloured parts of one model and its metadata; main() writes
    the OBJ."""

    def __init__(self, tolerance=0.01, angular=0.35):
        self.tolerance = tolerance
        self.angular = angular
        self.parts = []  # (shape, rgb, tolerance)
        self.slots = []  # (index, x, y, z) authoring space
        self.footprint_half = None
        self.anim_parts = []  # (name, axis, x, y, z)

    def add(self, shape, color="white", tolerance=None):
        """Add a part (a shape or a list of shapes) in color."""
        for s in _flat([shape]):
            self.parts.append((s, rgb(color), tolerance))
        return self

    def slot(self, index, x, y, z):
        """A seat or anchor point (# slot)."""
        self.slots.append((index, x, y, z))

    def footprint(self, half_x, half_y):
        """The model's footprint half-extents on the ground (# footprint)."""
        self.footprint_half = (half_x, half_y)

    def part(self, name, axis, x, y, z):
        """An animated sub-part loaded from name.obj, spinning about axis
        ("spin_y" or "spin_z") at pivot (x, y, z) (# part)."""
        self.anim_parts.append((name, axis, x, y, z))

    def write(self, path, source=""):
        def game(x, y, z):
            return (x, z, -y)

        lines = [f"# generated by models-src/lib/mogul.py from {source}"]
        for i, x, y, z in self.slots:
            gx, gy, gz = game(x, y, z)
            lines.append(f"# slot {i} {_g(gx)} {_g(gy)} {_g(gz)}")
        if self.footprint_half:
            lines.append(f"# footprint {_g(self.footprint_half[0])} {_g(self.footprint_half[1])}")
        for name, axis, x, y, z in self.anim_parts:
            gx, gy, gz = game(x, y, z)
            lines.append(f"# part {name} {axis} {_g(gx)} {_g(gy)} {_g(gz)}")

        nv = 0
        for shape, col, tol in self.parts:
            for solid in _solids(shape):
                verts, tris = solid.tessellate(tol or self.tolerance, self.angular)
                p = [game(v.X, v.Y, v.Z) for v in verts]
                for t in tris:
                    a, b, c = (p[i] for i in t)
                    n = _normal(a, b, c)
                    if n is None:
                        continue
                    for q in (a, b, c):
                        lines.append(f"v {_g(q[0])} {_g(q[1])} {_g(q[2])} {_g(col[0])} {_g(col[1])} {_g(col[2])}")
                    lines.append(f"vn {_g(n[0])} {_g(n[1])} {_g(n[2])}")
                    nv += 1
                    lines.append(f"f {3*nv-2}//{nv} {3*nv-1}//{nv} {3*nv}//{nv}")
        if nv == 0:
            raise SystemExit(f"{source}: no triangles")
        with open(path, "w") as f:
            f.write("\n".join(lines) + "\n")
        print(f"mogul: {nv} tris  {source} -> {path}")

    def main(self):
        """Write the OBJ named by the script's first argument."""
        source = os.path.relpath(sys.argv[0])
        if len(sys.argv) != 2:
            raise SystemExit(f"usage: python {source} <output.obj>")
        self.write(sys.argv[1], source)


def _solids(shape):
    if isinstance(shape, Compound):
        out = list(shape.solids())
        return out or [shape]
    return [shape]


def _normal(a, b, c):
    e1 = (b[0] - a[0], b[1] - a[1], b[2] - a[2])
    e2 = (c[0] - a[0], c[1] - a[1], c[2] - a[2])
    n = (e1[1] * e2[2] - e1[2] * e2[1], e1[2] * e2[0] - e1[0] * e2[2], e1[0] * e2[1] - e1[1] * e2[0])
    l = math.sqrt(n[0] ** 2 + n[1] ** 2 + n[2] ** 2)
    if l < 1e-12:
        return None
    return (n[0] / l, n[1] / l, n[2] / l)


def _g(x):
    """A number as OBJ text, like Go's %g at float32 precision."""
    x = float(x)
    if abs(x) < 5e-8:
        return "0"
    return f"{x:.6g}"

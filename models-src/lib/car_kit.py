"""Car kit — the parts every road vehicle shares (car_sedan.py,
car_minisuv.py, car_suv.py, car_jeep.py, car_van.py), which differ in
their proportions. world.CarKind names them; the renderer draws each kind
from its own mesh, tinted per car, with a roof rack or box
(car_roof_rack.py, car_roof_box.py) set on the roof at the height in
render.carRoofs.

    +X = forward (the car's front faces +X)
    +Y = lateral (driver's left when facing +X)
    +Z = up

The paint is near white so the per-car tint is the paint colour; glass,
tyres, trim and lights are dark or bright enough to read through any tint.

Cars are drawn by the hundred in parked lots, so the kit stays faceted:
the body, glass and roof are convex hulls of slabs whose corners are
rounded in plan (a few facets per corner) and whose top and bottom edges
are chamfered, and the wheels are 16-sided.

Conventions, units, and axis docs: see models-src/README.md.
"""

import numpy as np

from lib.mogul import *

paint = [0.92, 0.92, 0.92]
glass = [0.30, 0.40, 0.50]
trim = [0.16, 0.16, 0.17]  # bumpers, grille, rails, mirrors
tyre = [0.08, 0.08, 0.08]
hubcap = [0.55, 0.56, 0.58]
lamp = [1.00, 0.96, 0.82]
tail = [0.80, 0.12, 0.10]

wheel_fn = 16
corner_seg = 2  # facets per rounded corner


def ring(x0, x1, w, z, r=0.0, seg=corner_seg):
    """The outline of a rectangle x0..x1 by w (centred in Y) at height z,
    its corners rounded to r, as 3D points."""
    hw = w / 2
    r = min(r, (x1 - x0) / 2, hw)
    if r <= 1e-6:
        return [(x0, -hw, z), (x1, -hw, z), (x1, hw, z), (x0, hw, z)]
    pts = []
    for cx, cy, a0 in ((x1 - r, -hw + r, -90), (x1 - r, hw - r, 0), (x0 + r, hw - r, 90), (x0 + r, -hw + r, 180)):
        for k in range(seg + 1):
            a = math.radians(a0 + 90 * k / seg)
            pts.append((cx + r * math.cos(a), cy + r * math.sin(a), z))
    return pts


def slab(x0, x1, w, z, t=0.04, r=0.0):
    """Points of a thin box from x0 to x1, centred in Y, its top at z, its
    corners rounded in plan to r (for hulls)."""
    return ring(x0, x1, w, z - t, r) + ring(x0, x1, w, z, r)


def hull_of(*point_lists):
    """The convex hull of point lists, as a solid."""
    pts = [p for pl in point_lists for p in pl]
    return hull_points(np.array(pts, dtype=float))


def round_box(x0, x1, w, z0, z1, r):
    """A box x0..x1, centred in Y, z0..z1, corners rounded in plan."""
    return hull_of(ring(x0, x1, w, z0, r), ring(x0, x1, w, z1, r))


def wheel(m, x, y, r, t):
    """A tyre and hubcap, axle along Y, standing on the ground."""
    tyre_s = translate([x, y, r], rotate([90, 0, 0], cylinder(h=t, r=r, center=True, fn=wheel_fn)))
    hub_s = translate([x, y, r], rotate([90, 0, 0], cylinder(h=t + 0.02, r=r * 0.55, center=True, fn=10)))
    m.add(tyre_s, tyre)
    m.add(hub_s, hubcap)


def vehicle(m, len, w, r, wb, sill, belt, hood, nose, tailin, roof, gb, gr, axle_x=0, rails=False):
    """Add one car to model m.

    len, w        overall body length and width
    r, wb         wheel radius and wheelbase (axles centred on x = axle_x)
    sill          height of the body's underside
    belt          the beltline: top of the doors, bottom of the glass
    hood          top of the hood at the nose (below belt slopes it)
    nose, tailin  how far the top of the body is drawn in from the front
                  and back ends (rounding them)
    roof          roof height
    gb            (x0, x1) the glass at the beltline
    gr            (x0, x1) the glass at the roof
    rails         roof rails (SUVs)
    """
    L = len / 2
    # Body: a hull from the full-length underside to the beltline at the
    # back and the hood at the nose. The end slabs reach inward (inside
    # the hull) so their corners can round; the underside's bottom edge
    # and the beltline's top edge are chamfered.
    body = hull_of(
        ring(-L, L, w - 0.06, sill, 0.16),
        ring(-L, L, w, sill + 0.03, 0.18),
        ring(-L, L, w, sill + 0.12, 0.18),
        ring(-L + tailin, gb[0] + 0.1, w, belt - 0.04, 0.14),
        ring(-L + tailin, gb[0] + 0.1, w, belt - 0.02, 0.14),
        ring(-L + tailin + 0.02, gb[0] + 0.1, w - 0.05, belt, 0.12),
        ring(gb[1] - 0.1, L - nose, w - 0.06, hood - 0.04, 0.14),
        ring(gb[1] - 0.1, L - nose - 0.02, w - 0.10, hood, 0.12),
        slab(-L, -L + 0.30, w - 0.04, belt - 0.12, 0.04, 0.15),
        slab(L - 0.30, L, w - 0.08, hood - 0.12, 0.04, 0.15),
    )
    m.add(body, paint)
    # Glass greenhouse, and the painted roof on it.
    m.add(hull_of(slab(gb[0], gb[1], w - 0.10, belt + 0.02, 0.04, 0.08),
                  slab(gr[0], gr[1], w - 0.26, roof - 0.04, 0.04, 0.08)), glass)
    rx0, rx1, rw = gr[0] - 0.03, gr[1] + 0.03, w - 0.22
    m.add(hull_of(ring(rx0, rx1, rw, roof - 0.06, 0.10),
                  ring(rx0, rx1, rw, roof - 0.025, 0.10),
                  ring(rx0 + 0.03, rx1 - 0.03, rw - 0.05, roof, 0.08)), paint)

    # Pillars down the sides of the glass: the windshield's (A), between
    # the doors (B), and a broad one at the back (C).
    def pillar(b0, b1, r0, r1, s):
        return hull(translate([b0, s * (w - 0.10) / 2 - 0.03, belt], cube([b1 - b0, 0.06, 0.02])),
                    translate([r0, s * (w - 0.26) / 2 - 0.03, roof - 0.06], cube([r1 - r0, 0.06, 0.02])))

    gbm, grm = (gb[0] + gb[1]) / 2, (gr[0] + gr[1]) / 2
    for s in (-1, 1):
        m.add([pillar(gb[1] - 0.14, gb[1], gr[1] - 0.10, gr[1], s),
               pillar(gbm - 0.06, gbm + 0.06, grm - 0.06, grm + 0.06, s),
               pillar(gb[0], gb[0] + 0.30, gr[0], gr[0] + 0.24, s)], paint)
    if rails:
        for s in (-1, 1):
            m.add(translate([gr[0] + 0.05, s * (w / 2 - 0.2) - 0.03, roof],
                            cube([gr[1] - gr[0] - 0.1, 0.06, 0.06])), trim)

    # Wheels, flush with the body sides.
    t = 0.24
    for x in (axle_x + wb / 2, axle_x - wb / 2):
        for s in (-1, 1):
            wheel(m, x, s * (w / 2 - t / 2 + 0.02), r, t)

    # Bumpers (wrapped round the corners), grille, lights.
    bz = sill + 0.02
    m.add([round_box(L - 0.08, L + 0.06, w - 0.04, bz, bz + 0.22, 0.06),
           round_box(-L - 0.06, -L + 0.08, w - 0.04, bz, bz + 0.22, 0.06),
           translate([L - 0.03, -w * 0.25, bz + 0.24], cube([0.05, w * 0.5, max(hood - bz - 0.42, 0.12)]))], trim)
    lz = hood - 0.16
    for s in (-1, 1):
        m.add(translate([L - 0.04, s * (w / 2 - 0.22) - 0.14, lz - 0.08], cube([0.06, 0.28, 0.13])), lamp)
        m.add(translate([-L - 0.02, s * (w / 2 - 0.16) - 0.10, belt - 0.30], cube([0.06, 0.20, 0.16])), tail)
    # Mirrors.
    for s in (-1, 1):
        m.add(translate([gb[1] - 0.25, s * (w / 2 + 0.06) - 0.06, belt + 0.02], cube([0.12, 0.12, 0.10])), trim)

    m.footprint(len / 2, w / 2 + 0.08)

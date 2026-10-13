"""Snowcat / piste basher — the over-snow tracked vehicle that grooms
trails. Read at a glance: long parallel tracks, a tall cab forward of
centre with a wraparound windshield, a V-shaped blade pushed out front
for snow piling, and a wide tiller hanging off the back that does the
actual corduroy work.

Real specs (Pisten Bully 600 class):
  length 7 m  ·  width with tracks 4 m  ·  height 3 m  ·  blade 5 m
We dial the blade in a touch under that so the silhouette doesn't scream
"monster truck" at low zoom.

Origin at the centre of the footprint on Z = 0. +X is forward (direction
of travel), +Y is the lateral axis.

The Python port keeps the OpenSCAD boxes' extents but rounds the edges
that read as moulded: the track ends curve round their idler wheels, the
cab's corners and the hull's long top edges are rounded, and the tiller drum
is rounded fore and aft.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
hull_l = 5.0  # hull length (X) — tracks extend beyond this
hull_w = 2.4  # hull width (Y)
hull_h = 1.4  # hull thickness in Z (top of hull above ground)

# Tracks — running along ±Y, full length plus some overhang fore/aft.
track_l = 6.0  # longer than the hull so the cat reads as tracked, not wheeled
track_w = 0.7  # track belt width (Y)
track_h = 0.6  # track height (Z) — half-buried look at the deck level

# Cab — operator station above and slightly forward of the hull centre.
cab_l = 2.0
cab_w = 2.0
cab_h = 1.4
cab_x = 0.2  # slight forward bias — operator sees the blade

# Cab roof — a thin slab cap
roof_thk = 0.08

# Front blade — angled V-pusher mounted on a short arm out front.
blade_w = 4.4  # width across (Y)
blade_h = 1.2  # height (Z)
blade_thk = 0.15
blade_v_offset = 0.6  # how far the centre of the V is pulled back vs the wings
arm_l = 0.8  # arm extending from the hull front to the blade
arm_w = 0.25
arm_h = 0.25

# Rear tiller — the wide drum-and-comb assembly that lays corduroy.
tiller_w = 3.6
tiller_l = 0.9
tiller_h = 0.5
tiller_x = -hull_l / 2 - tiller_l / 2 - 0.1  # a touch behind the hull


def rounded_box(size, r, axis, pos, k=4, top_only=False):
    """A centred box at pos with the four edges running along axis ("X",
    "Y" or "Z") rounded to r, each round faceted into k flat strips
    (cheaper than a build123d fillet, which tessellates densely).
    top_only rounds just the two upper edges (axis "X" or "Y"). Local
    helper: mogul.py has no rounded box."""
    sx, sy, sz = size
    # The section perpendicular to axis, as (u, v) half-extents, and how
    # to map the section's (u, v, w) back to (x, y, z).
    if axis == "Z":
        hu, hv, length = sx / 2, sy / 2, sz
    elif axis == "Y":
        hu, hv, length = sx / 2, sz / 2, sy
    else:
        hu, hv, length = sy / 2, sz / 2, sx
    pts = []
    for cu, cv, a0 in ((1, 1, 0), (-1, 1, 90), (-1, -1, 180), (1, -1, 270)):
        if top_only and cv < 0:
            pts.append((cu * hu, cv * hv))
            continue
        ox, oy = cu * (hu - r), cv * (hv - r)
        for i in range(k + 1):
            a = math.radians(a0 + 90 * i / k)
            pts.append((ox + r * math.cos(a), oy + r * math.sin(a)))
    slab = linear_extrude(pts, length, center=True)
    if axis == "Y":
        slab = rotate([90, 0, 0], slab)  # section (x, z), extruded along Y
    elif axis == "X":
        slab = rotate([90, 0, 90], slab)  # section (y, z), extruded along X
    return translate(pos, slab)


m = Model()

# ── Tracks ─────────────────────────────────────────────────────────────
# Ends rounded about the idler axles (edges along Y).
for side in (+1, -1):
    track_y = side * (hull_w / 2 + track_w / 2 - 0.05)
    m.add(rounded_box([track_l, track_w, track_h], 0.25, "Y", [0, track_y, track_h / 2]), "DimGray")

# ── Hull ───────────────────────────────────────────────────────────────
# Safety-orange like a real snowcat; top edges softened.
m.add(rounded_box([hull_l, hull_w, hull_h], 0.12, "X", [0, 0, track_h + hull_h / 2], k=2, top_only=True), "OrangeRed")

# ── Cab ────────────────────────────────────────────────────────────────
cab_z = track_h + hull_h + cab_h / 2
m.add(rounded_box([cab_l, cab_w, cab_h], 0.22, "Z", [cab_x, 0, cab_z]), "SteelBlue")
# Roof slab — slightly oversized so it reads as a rain cap; corners
# rounded to follow the cab.
m.add(
    rounded_box(
        [cab_l + 0.15, cab_w + 0.15, roof_thk], 0.28, "Z", [cab_x, 0, track_h + hull_h + cab_h + roof_thk / 2]
    ),
    "DimGray",
)

# ── Blade ──────────────────────────────────────────────────────────────
# A V whose wings angle forward of the centre: two thin cuboids rotated
# about the V apex so each wing meets at the centreline.
blade_x = hull_l / 2 + arm_l + blade_v_offset
# Support arm from hull front out to the blade centre
m.add(translate([hull_l / 2 + arm_l / 2, 0, track_h + arm_h / 2], cube([arm_l, arm_w, arm_h], center=True)), "DimGray")


def blade_wing(side):
    wing_l = blade_w / 2
    # Wings rake slightly back from the centre.
    return rotate([0, 0, side * -15], translate([0, side * wing_l / 2, blade_h / 2], cube([blade_thk, wing_l, blade_h], center=True)))


m.add([translate([blade_x, 0, track_h], blade_wing(s)) for s in (+1, -1)], "Goldenrod")

# ── Tiller ─────────────────────────────────────────────────────────────
# The corduroy-laying drum hanging off the back, rounded fore and aft
# (edges along Y).
m.add(rounded_box([tiller_l, tiller_w, tiller_h], 0.2, "Y", [tiller_x, 0, tiller_h / 2]), "DimGray")
# A few visible "fingers" suggesting the comb behind the drum.
m.add(
    [translate([tiller_x - tiller_l / 2 - 0.15, i * 0.7, tiller_h * 0.3], cube([0.30, 0.10, 0.10], center=True)) for i in range(-2, 3)],
    "DarkGray",
)

m.main()

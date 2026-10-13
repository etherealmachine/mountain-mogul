"""Snowmobile — ski-patrol rescue sled.

A compact personal over-snow vehicle: single centred rear track, two
front steering skis, a tapered hood over the engine, a seat, and
handlebars. Much narrower and shorter than the snowcat.

Real specs (Arctic Cat / Ski-Doo touring class):
  length ~2.9 m  ·  width ~0.70 m  ·  height ~1.1 m to bar tops

Body colours are near-white so the renderer's per-instance ColorTint
controls the appearance cleanly: white tint = parked, red tint = active.

The Python port keeps the OpenSCAD boxes' extents and softens what reads
as moulded: the track's ends curve round its idlers, the seat's top edges
are rounded, and the grips are round.

Origin at the centre of the footprint on Z=0. +X is forward.
Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

# ── Dimensions (metres) ───────────────────────────────────────────────────

# Rear track — single centred belt
track_l = 2.50
track_w = 0.42
track_h = 0.22

# Chassis / tunnel that everything mounts to
chassis_l = 2.55
chassis_w = 0.62
chassis_h = 0.26

# Hood — tapers from the firewall (where chassis meets engine bay) to nose tip
hood_l = 1.20  # measured forward from the firewall
hood_w_back = 0.60  # width at the firewall (matches chassis width)
hood_w_nose = 0.16  # width at the nose tip
hood_h = 0.46  # hood height above chassis top

# Front skis — two wide-set steering skis; their centres live fore of
# the chassis nose so the tips visibly extend forward.
ski_l = 1.05
ski_w = 0.13
ski_h = 0.06
ski_span = 0.38  # half-span: ski centreline to vehicle centreline
ski_tip_fwd = 0.30  # how far the ski tip extends ahead of the chassis nose

# Seat — raised saddle slightly rearward of the chassis centre
seat_l = 0.80
seat_w = 0.28
seat_h = 0.16
seat_x = -0.20

# Windshield — raked slab ahead of the handlebars
ws_h = 0.30
ws_thk = 0.05
ws_rake = 22  # degrees forward from vertical

# Handlebars — T-bar mounted above the windshield
hbar_w = 0.64
hbar_thk = 0.04
hbar_stem = 0.28  # stem height above chassis top

# ── Derived positions ─────────────────────────────────────────────────────
chassis_front = chassis_l / 2
chassis_back = -chassis_l / 2
firewall_x = chassis_front - hood_l  # where the hood starts

z_chassis = track_h
z_top = z_chassis + chassis_h

ski_x = chassis_front + ski_tip_fwd - ski_l / 2

hbar_x = firewall_x + 0.28  # handlebars live above the firewall area


def rounded_xz(size, r, pos, k=4, top_only=False):
    """A centred box at pos with its four edges along Y (or just the two
    upper ones) rounded to r, each faceted into k strips. Local helper:
    mogul.py has no rounded box, and a build123d fillet tessellates far
    denser than these small parts need."""
    hx, hz = size[0] / 2, size[2] / 2
    pts = []
    for cu, cv, a0 in ((1, 1, 0), (-1, 1, 90), (-1, -1, 180), (1, -1, 270)):
        if top_only and cv < 0:
            pts.append((cu * hx, cv * hz))
            continue
        ox, oz = cu * (hx - r), cv * (hz - r)
        for i in range(k + 1):
            a = math.radians(a0 + 90 * i / k)
            pts.append((ox + r * math.cos(a), oz + r * math.sin(a)))
    # Section in (x, z), extruded along Y.
    return translate(pos, rotate([90, 0, 0], linear_extrude(pts, size[1], center=True)))


m = Model()

# ── Track ─────────────────────────────────────────────────────────────────
# Ends rounded about the front and rear idlers.
m.add(rounded_xz([track_l, track_w, track_h], 0.10, [0, 0, track_h / 2]), "DimGray")
# Three cross-cleats suggest the tread pattern.
m.add([translate([dx, 0, track_h + 0.01], cube([0.14, track_w + 0.06, 0.03], center=True)) for dx in (-0.75, 0, 0.75)], [0.35, 0.35, 0.35])

# ── Chassis ───────────────────────────────────────────────────────────────
m.add(translate([0, 0, z_chassis + chassis_h / 2], cube([chassis_l, chassis_w, chassis_h], center=True)), [0.93, 0.93, 0.95])

# ── Hood ──────────────────────────────────────────────────────────────────
# A convex hull from the wide firewall section to the narrow nose tip,
# a taper that reads clearly as a snowmobile's engine cover.
m.add(
    hull(
        # Nose tip — very narrow
        translate([chassis_front + 0.06, 0, z_top + hood_h * 0.20], cube([0.06, hood_w_nose, hood_h * 0.40], center=True)),
        # Firewall — full width, full height
        translate([firewall_x - 0.04, 0, z_top + hood_h * 0.50], cube([0.08, hood_w_back, hood_h], center=True)),
    ),
    [0.90, 0.90, 0.92],
)

# ── Skis and struts ───────────────────────────────────────────────────────
tip_rise = 0.08  # skis are horizontal with a slight tip upward at the front
strut_bot = ski_h + 0.02
strut_h = z_chassis - strut_bot
for side in (+1, -1):
    m.add(
        translate([ski_x, side * ski_span, ski_h / 2], rotate([0, -math.degrees(math.atan(tip_rise / ski_l)), 0], cube([ski_l, ski_w, ski_h], center=True))),
        [0.84, 0.84, 0.88],
    )
    # Ski leg strut from chassis underside down to the ski
    m.add(translate([ski_x + 0.12, side * ski_span, strut_bot + strut_h / 2], cube([0.08, 0.06, strut_h], center=True)), "Gray")

# ── Seat ──────────────────────────────────────────────────────────────────
# Padded saddle: upper edges rounded.
m.add(rounded_xz([seat_l, seat_w, seat_h], 0.06, [seat_x, 0, z_top + seat_h / 2], k=3, top_only=True), "DarkGray")

# ── Windshield ────────────────────────────────────────────────────────────
m.add(
    translate([hbar_x + 0.10, 0, z_top + ws_h / 2], rotate([0, ws_rake, 0], cube([ws_thk, chassis_w * 0.85, ws_h], center=True))),
    [0.75, 0.90, 0.95],
)

# ── Handlebars ────────────────────────────────────────────────────────────
bar_z = z_top + hbar_stem
# Vertical stem
m.add(translate([hbar_x, 0, z_top + hbar_stem / 2], cube([hbar_thk, hbar_thk, hbar_stem], center=True)), "Gray")
# Horizontal crossbar
m.add(translate([hbar_x, 0, bar_z], cube([hbar_thk * 2.5, hbar_w, hbar_thk], center=True)), "Gray")
# Grip ends — round, along Y, as long and as thick as the old blocks.
for side in (-1, +1):
    y0 = side * hbar_w / 2
    m.add(rod([hbar_x, y0 - 0.03, bar_z], [hbar_x, y0 + 0.03, bar_z], hbar_thk * 0.75, fn=10), [0.20, 0.20, 0.20])

m.main()

"""Ticket office — the day-ticket and season-pass window at the base area.
A small timber booth, long across its front so several guests can be
served side by side: three service windows on the +X façade under a red
awning, a counter ledge beneath them, and a sign board on the roof so the
building reads as "tickets" from the top-down camera.

Conventions, units, and axis docs: see models-src/README.md.

    +X = front façade (service windows face +X)
    +Y = lateral / perpendicular horizontal
    +Z = up (the writer turns it to game Y-up)
  Origin sits at ground plane, centred on the footprint.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
booth_d = 4.0  # depth, front to back along X
booth_w = 8.0  # width along Y — the service frontage
booth_h = 3.2  # wall height at the front

# Mono-pitch roof falling toward the back so snow slides off away from
# the queue.
roof_drop = 0.8
roof_overhang = 0.4
roof_t = 0.25

# Service windows on the front face.
window_count = 3
window_w = 1.6
window_h = 1.2
window_sill = 1.1  # counter height
window_inset = 0.05

# Counter ledge and awning over the windows.
ledge_depth = 0.35
awning_depth = 1.2
awning_height = 2.55
awning_t = 0.12

# Roof sign.
sign_w = 5.0
sign_h = 1.1
sign_t = 0.2


def bar_y(a, b, length, r):
    """A bar along Y, centred, its a × b (X × Z) section rounded to r."""
    return rotate([90, 0, 0], linear_extrude(rounded_rect(a, b, r, seg=2), height=length, center=True))


m = Model()

# ── Geometry ───────────────────────────────────────────────────────────
# Walls: a box with its corners rounded in plan, its top cut to the roof
# pitch (front booth_h, back roof_drop lower).
walls = linear_extrude(rounded_rect(booth_d, booth_w, 0.15, seg=2), height=booth_h)
pitch = math.degrees(math.atan(roof_drop / booth_d))
cutter = translate([0, 0, 2], cube([4 * booth_d, 2 * booth_w, 4], center=True))  # above z = 0
walls = difference(walls, translate([booth_d / 2, 0, booth_h], rotate([0, -pitch, 0], cutter)))
m.add(walls, "Sienna")

# Mono-pitch slab: a thin box tilted about Y so its underside meets the
# wall tops, high at the front and falling to the back; its front and back
# edges rounded.
run = booth_d + 2 * roof_overhang
angle = math.degrees(math.atan(roof_drop / booth_d))
roof = bar_y(run / math.cos(math.radians(angle)), roof_t, booth_w + 2 * roof_overhang, 0.08)
m.add(translate([0, 0, booth_h - roof_drop / 2 + roof_t / 2], rotate([0, -angle, 0], roof)), "DarkSlateGray")


def window_y(i):
    return (i - (window_count - 1) / 2) * (booth_w / window_count)


m.add([translate([booth_d / 2 + window_inset / 2, window_y(i), window_sill + window_h / 2],
                 cube([window_inset, window_w, window_h], center=True)) for i in range(window_count)],
      "LightSkyBlue")

# Counter ledge.
m.add(translate([booth_d / 2 + ledge_depth / 2, 0, window_sill - 0.05],
                cube([ledge_depth, booth_w - 0.6, 0.1], center=True)), "Tan")

# Awning over the windows, its edges rounded.
m.add(translate([booth_d / 2 + awning_depth / 2, 0, awning_height],
                rotate([0, 12, 0], bar_y(awning_depth, awning_t, booth_w - 0.3, 0.05))), "FireBrick")

# Sign board on posts along the roof's front edge, facing +X.
post = 0.15
base = booth_h - 0.2  # posts start inside the sloped roof slab
lift = 0.8
m.add([translate([booth_d / 2 - 0.6, y, base + lift / 2], cube([post, post, lift], center=True))
       for y in (-sign_w / 2 + 0.4, sign_w / 2 - 0.4)], "DimGray")
m.add(translate([booth_d / 2 - 0.6, 0, base + lift + sign_h / 2],
                rotate([90, 0, 90], linear_extrude(rounded_rect(sign_w, sign_h, 0.12, seg=2), height=sign_t, center=True))), "Gold")

# ── Footprint metadata ─────────────────────────────────────────────────
# Half-extents in authoring coords (X, Y). The awning projects
# awning_depth past the front wall; use that side for halfX so the apron
# covers it.
m.footprint(booth_d / 2 + awning_depth, booth_w / 2 + roof_overhang)
m.main()

"""Rail kit — what every train shares: bogies and wheels, buffers,
car bodies, window bands, roof sections and pantographs (steam_loco.py,
steam_coach.py, electric_loco.py, electric_coach.py, hsr_power.py,
hsr_coach.py, funicular_car.py).

    +X = along the track (a car's front, or its A end, faces +X)
    +Y = across (left facing +X)
    +Z = up, from the top of the rail (z = 0)

Standard gauge: wheel treads 0.7175 either side of the centre line.
Trains are drawn a few at a time, so the kit is moderately detailed;
wheels are 14-sided.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

gauge_half = 0.7175
steel = [0.22, 0.22, 0.24]
dark = [0.10, 0.10, 0.11]
glass = [0.16, 0.22, 0.28]
grey = [0.55, 0.56, 0.58]
silver = [0.75, 0.76, 0.78]
wheel_fn = 14


def box(x0, x1, w, z0, z1, y=0.0):
    """A box from x0 to x1 along the track, w across centred on y, from
    z0 up to z1."""
    return translate([x0, y - w / 2, z0], cube([x1 - x0, w, z1 - z0]))


def body(x0, x1, w, z0, z1, r=0.25):
    """A car body: x0 to x1, w across, z0 to z1, its corners rounded in
    plan to r."""
    return translate([(x0 + x1) / 2, 0, z0], linear_extrude(rounded_rect(x1 - x0, w, r), z1 - z0))


def section(profile, x0, x1):
    """A shape of a cross-section profile (y, z points) run along X from
    x0 to x1."""
    return translate([x0, 0, 0], rotate([90, 0, 90], linear_extrude(profile, x1 - x0)))


def roof_profile(w, z0, rise, seg=5):
    """A curved roof's cross-section: w across, from z0 up rise at the
    middle, as (y, z) points."""
    return [(-w / 2 * math.cos(math.pi * k / seg), z0 + rise * math.sin(math.pi * k / seg)) for k in range(seg + 1)]


def wheelset(m, x, r, color=steel):
    """An axle's two wheels at x, radius r, on the rails."""
    for s in (-1, 1):
        m.add(translate([x, s * gauge_half, r], rotate([90, 0, 0], cylinder(h=0.13, r=r, center=True, fn=wheel_fn))), color)
    m.add(translate([x, 0, r], rotate([90, 0, 0], cylinder(h=2 * gauge_half, r=0.08, center=True, fn=8))), dark)


def bogie(m, x, base=2.5, r=0.46, frame=dark):
    """A two-axle bogie centred at x: its wheels base apart and a side
    frame either side."""
    for dx in (-base / 2, base / 2):
        wheelset(m, x + dx, r)
    for s in (-1, 1):
        m.add(box(x - base / 2 - 0.5, x + base / 2 + 0.5, 0.18, r - 0.15, r + 0.25, y=s * (gauge_half + 0.2)), frame)
    m.add(box(x - 0.6, x + 0.6, 2.0, r + 0.2, r + 0.45), frame)  # the bolster


def buffers(m, x, out, z=1.06, beam=None, beam_w=2.6):
    """Buffers on the end at x, facing out (+1 or -1), at height z; with
    a buffer beam in colour beam."""
    if beam is not None:
        m.add(box(min(x, x - out * 0.2), max(x, x - out * 0.2), beam_w, z - 0.3, z + 0.3), beam)
    for s in (-1, 1):
        m.add(translate([x + out * 0.22, s * 0.875, z], rotate([0, 90, 0], cylinder(h=0.44, r=0.09, center=True, fn=8))), steel)
        m.add(translate([x + out * 0.46, s * 0.875, z], rotate([0, 90, 0], cylinder(h=0.06, r=0.19, center=True, fn=12))), steel)
    m.add(translate([x + out * 0.25, 0, z], rotate([0, 90, 0], cylinder(h=0.5, r=0.07, center=True, fn=6))), dark)


def window_band(m, x0, x1, w, z0, z1, gap=0.0, pane=0.0, color=glass):
    """Windows on both sides from x0 to x1 between z0 and z1: one band, or
    panes pane long with gap between them."""
    t = 0.04
    for s in (-1, 1):
        y = s * (w / 2 + t / 2 - 0.01)
        if pane <= 0:
            m.add(box(x0, x1, t, z0, z1, y=y), color)
            continue
        x = x0
        while x + pane <= x1 + 1e-6:
            m.add(box(x, x + pane, t, z0, z1, y=y), color)
            x += pane + gap


def stripe(m, x0, x1, w, z0, z1, color):
    """A livery stripe on both sides."""
    window_band(m, x0, x1, w, z0, z1, color=color)


def pantograph(m, x, z, color=grey):
    """A single-arm pantograph on the roof at x, roof height z, folded
    half up."""
    m.add(box(x - 0.6, x + 0.6, 1.0, z, z + 0.12), dark)  # its base frame
    m.add(rod([x - 0.4, 0, z + 0.12], [x + 0.5, 0, z + 0.9], 0.04, fn=6), color)
    m.add(rod([x + 0.5, 0, z + 0.9], [x - 0.2, 0, z + 1.6], 0.035, fn=6), color)
    m.add(box(x - 0.3, x - 0.1, 1.7, z + 1.55, z + 1.65), color)  # the head

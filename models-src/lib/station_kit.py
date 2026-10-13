"""Station kit — the parts the lift stations share (lift_station.py,
lift_terminal.py, lift_terminal_wide.py): a housing about as wide as the
chairs with the cable and bullwheel inside it, its underside a little
over the skiers' heads; round posts just outside the chairs' sweep; the
red load lines; and the operator's hut.

Everything that stands on the ground runs sink metres below it, so a
station on ground that isn't quite level never shows a gap under a post.

Station frame: +X from the bullwheel toward the other end, +Y across
(the hut's side), +Z up; see lift_station.py.
"""

from lib.mogul import *

cable_h = 3.65  # world.BullwheelHeight: cable height at loading
cable_gap = 1.50  # world.CableGap: up / down cable either side of the axis
sink = 1.0  # how far posts, footings and huts run below the ground



def sweep_half(seat_w):
    """How far out from the axis the widest chair reaches: the cable gap
    and half its seat."""
    return cable_gap + seat_w / 2


# The housing over a station's line: its underside a little over the
# skiers' heads, the cable, rail and bullwheel inside it, the chairs
# hanging out of slots along each cable.
housing_bottom = 2.6  # underside: about 0.9 m over a standing skier
housing_wall = 3.95  # where the sides turn into the vault, over the cable (3.65 m)
housing_rise = 0.4  # the vault's crown above that


def housing_profile(half, bottom, wall, rise, r=0.25, seg=10):
    """The housing's cross-section (y, z): rounded along the bottom edges,
    straight sides, a shallow vault over the top."""
    pts = []
    # Bottom right corner, round from the underside up the side.
    for k in range(seg // 2 + 1):
        a = -math.pi / 2 + (math.pi / 2) * k / (seg // 2)
        pts.append((half - r + r * math.cos(a), bottom + r + r * math.sin(a)))
    # The vault from the right side over to the left.
    c = (half * half - rise * rise) / (2 * rise)
    R = c + rise
    a0 = math.atan2(c, half)
    for k in range(seg + 1):
        a = a0 + (math.pi - 2 * a0) * k / seg
        pts.append((R * math.cos(a), wall - c + R * math.sin(a)))
    # Bottom left corner.
    for k in range(seg // 2 + 1):
        a = math.pi + (math.pi / 2) * k / (seg // 2)
        pts.append((-half + r + r * math.cos(a), bottom + r + r * math.sin(a)))
    return pts


def add_housing(m, x0, x1, half):
    """The housing from x0 to x1, half wide either side of the axis, with a
    blue band round its foot and ends, and dark slots along its underside
    where the chairs hang through."""
    length = x1 - x0
    body = rotate([90, 0, 90], linear_extrude(housing_profile(half, housing_bottom, housing_wall, housing_rise),
                                              height=length, center=True))
    m.add(translate([(x0 + x1) / 2, 0, 0], body), "Gainsboro")
    ends = [translate([x, 0, 0], rotate([90, 0, 90], linear_extrude(
        housing_profile(half + 0.04, housing_bottom - 0.04, housing_wall, housing_rise + 0.04), height=0.1, center=True)))
        for x in (x0, x1)]
    band = [translate([x0, s * (half + 0.02) - 0.02, housing_bottom + 0.05], cube([length, 0.04, 0.28])) for s in (-1, 1)]
    m.add(ends + band, "SteelBlue")
    slots = [translate([x0 + 0.1, y - 0.15, housing_bottom - 0.01], cube([length - 0.2, 0.3, 0.02]))
             for y in (-cable_gap, cable_gap)]
    m.add(slots, [0.18, 0.19, 0.21])
    # Round the back, the slots follow the chairs round the bullwheel.
    m.add(translate([0, 0, housing_bottom - 0.01], difference(cylinder(h=0.02, r=cable_gap + 0.15),
                                                              cylinder(h=0.04, r=cable_gap - 0.15, center=True))),
          [0.18, 0.19, 0.21])


def add_post(m, x, y, top, r=0.16):
    """A round post from below the ground up to top, on a footing."""
    m.add(translate([x, y, -sink], cylinder(h=top + sink, r=r, fn=12)), "SlateGray")
    m.add(translate([x, y, -sink], cylinder(h=sink + 0.3, r=r + 0.2, fn=12)), "Silver")


def add_load_lines(m, x, length=3.2):
    """The red load line on the snow under each cable (riders get on over
    the -Y one at the base and off over the +Y one at the top)."""
    m.add([translate([x - 0.08, y - length / 2, 0.02], cube([0.16, length, 0.03])) for y in (-cable_gap, cable_gap)],
          "Crimson")


def add_hut(m, x, y, door_back=True, l=2.4, w=1.9, h=2.4):
    """The operator's hut centred at (x, y): windows on its -Y side, facing
    the line, and its +X end; the door on its -X end (or +X)."""
    m.add(translate([x, y, -sink], linear_extrude(rounded_rect(l, w, 0.08, seg=2), height=h + sink)), "Sienna")
    m.add([translate([x, y - w / 2 - 0.01, h * 0.62], cube([l * 0.7, 0.04, 0.8], center=True)),
           translate([x + (l / 2 + 0.01) * (1 if door_back else -1), y, h * 0.62], cube([0.04, w * 0.6, 0.8], center=True))],
          "LightSteelBlue")
    m.add(translate([x - (l / 2 + 0.01) * (1 if door_back else -1), y + 0.2, 0.95], cube([0.04, 0.8, 1.9], center=True)),
          "SaddleBrown")
    m.add(translate([x, y, 0], hull(
        translate([0, -0.15, h + 0.05], cube([l + 0.5, w + 0.5, 0.1], center=True)),
        translate([0, 0.1, h + 0.35], cube([l + 0.5, 0.2, 0.1], center=True)))), "DarkSlateGray")


# ── Detachable terminal ────────────────────────────────────────────────
# A long housing on four posts, after a Doppelmayr terminal like Red
# Dog's: the bullwheel, the haul rope and the rail inside it, and the
# chairs, off the rope, hanging out of its underside at loading height
# round the back, along the riders' side to the load point, and on
# through the front of the terminal, slow all the way
# (world.StationGeom: SlowZone). The operator's booth stands off the front
# corner on the empty chairs' side.

term_load_x = 4.5  # world.StationGeom.LoadAhead
term_back = -2.6
term_front = 12.0
term_post_xs = (2.0, 11.0)
term_hut_x = 13.2


def term_post_y(seat_w):
    """Where a terminal's posts stand across the axis: just outside the
    sweep of its widest chair."""
    return sweep_half(seat_w) + 0.45


def term_hut_y(seat_w):
    return term_post_y(seat_w) + 1.4


def add_terminal(m, seat_w):
    """A detachable terminal for chairs seat_w wide."""
    from build123d import Box, fillet

    post_y = term_post_y(seat_w)
    add_housing(m, term_back, term_front, post_y + 0.2)
    # Drive housing on top, over the wheel.
    drive = fillet(Box(2.6, 2.0, 0.7).edges().filter_by(lambda e: e.center().Z > 0.2), 0.15)
    m.add(translate([-0.3, 0, housing_wall + housing_rise * 0.8 + 0.3], drive), "DimGray", tolerance=0.03)

    for x in term_post_xs:
        for y in (-post_y, post_y):
            add_post(m, x, y, housing_bottom + 0.3, r=0.2)
    add_load_lines(m, term_load_x, length=seat_w + 0.6)
    add_hut(m, term_hut_x, term_hut_y(seat_w), door_back=False)

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



def sweep_half(seat_w, gap=cable_gap):
    """How far out from the axis the widest chair reaches: the cable gap
    and half its seat."""
    return gap + seat_w / 2


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


def add_housing(m, x0, x1, half, bottom=housing_bottom, wall=housing_wall, rise=housing_rise,
                band_color="SteelBlue", slots=True, gap=cable_gap):
    """The housing from x0 to x1, half wide either side of the axis, with a
    band round its foot and ends, and (slots) dark slots along its
    underside where the chairs hang through."""
    length = x1 - x0
    body = rotate([90, 0, 90], linear_extrude(housing_profile(half, bottom, wall, rise),
                                              height=length, center=True))
    m.add(translate([(x0 + x1) / 2, 0, 0], body), "Gainsboro")
    ends = [translate([x, 0, 0], rotate([90, 0, 90], linear_extrude(
        housing_profile(half + 0.04, bottom - 0.04, wall, rise + 0.04), height=0.1, center=True)))
        for x in (x0, x1)]
    band = [translate([x0, s * (half + 0.02) - 0.02, bottom + 0.05], cube([length, 0.04, 0.28])) for s in (-1, 1)]
    m.add(ends + band, band_color)
    if not slots:
        return
    slots = [translate([x0 + 0.1, y - 0.15, bottom - 0.01], cube([length - 0.2, 0.3, 0.02]))
             for y in (-gap, gap)]
    m.add(slots, [0.18, 0.19, 0.21])
    # Round the back, the slots follow the chairs round the bullwheel.
    m.add(translate([0, 0, bottom - 0.01], difference(cylinder(h=0.02, r=gap + 0.15),
                                                      cylinder(h=0.04, r=gap - 0.15, center=True))),
          [0.18, 0.19, 0.21])


def add_post(m, x, y, top, r=0.16):
    """A round post from below the ground up to top, on a footing."""
    m.add(translate([x, y, -sink], cylinder(h=top + sink, r=r, fn=12)), "SlateGray")
    m.add(translate([x, y, -sink], cylinder(h=sink + 0.3, r=r + 0.2, fn=12)), "Silver")


def add_load_lines(m, x, length=3.2, gap=cable_gap, z=0.02):
    """The red load line on the snow under each cable (riders get on over
    the -Y one at the base and off over the +Y one at the top)."""
    m.add([translate([x - 0.08, y - length / 2, z], cube([0.16, length, 0.03])) for y in (-gap, gap)],
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


def term_post_y(seat_w, gap=cable_gap):
    """Where a terminal's posts stand across the axis: just outside the
    sweep of its widest chair."""
    return sweep_half(seat_w, gap) + 0.45


def term_hut_y(seat_w, gap=cable_gap):
    return term_post_y(seat_w, gap) + 1.4


def add_terminal(m, seat_w, gap=cable_gap):
    """A detachable terminal for chairs seat_w wide, their cables gap
    either side of the axis."""
    from build123d import Box, fillet

    post_y = term_post_y(seat_w, gap)
    back = min(term_back, -gap - 1.1)
    add_housing(m, back, term_front, post_y + 0.2, gap=gap)
    # Drive housing on top, over the wheel.
    drive = fillet(Box(2.6, 2.0, 0.7).edges().filter_by(lambda e: e.center().Z > 0.2), 0.15)
    m.add(translate([-0.3, 0, housing_wall + housing_rise * 0.8 + 0.3], drive), "DimGray", tolerance=0.03)

    for x in term_post_xs:
        for y in (-post_y, post_y):
            add_post(m, x, y, housing_bottom + 0.3, r=0.2)
    add_load_lines(m, term_load_x, length=seat_w + 0.6, gap=gap)
    add_hut(m, term_hut_x, term_hut_y(seat_w, gap), door_back=False)


# ── Station house ──────────────────────────────────────────────────────
# The gondola's and the tram's terminals (gondola_terminal.py,
# tram_terminal.py): the cabins dock on a platform up over an enclosed
# ground floor, which the line files into, and riders getting off walk
# out of, by a lobby in the middle of its back wall (world.StationGeom:
# Enclosed, DoorHalf).

house_wall = [0.47, 0.49, 0.53]
house_deck = [0.62, 0.63, 0.65]
house_glass = [0.30, 0.40, 0.48]
house_red = [0.74, 0.11, 0.13]


def add_station_house(m, x0, x1, half, floor, door_half, door_h=2.0, lobby=2.4):
    """The ground floor from x0 (its back, the way in) to x1, half wide
    either side of the axis and floor high, its top the platform: a band
    of windows down each side, a red fascia round the platform's edge, and
    the lobby, door_half either side of the axis and door_h high, set
    lobby deep into the back wall, its glass doors at the far end."""
    block = translate([(x0 + x1) / 2, 0, (floor - sink) / 2], cube([x1 - x0, 2 * half, floor + sink], center=True))
    void = translate([x0 - 0.1, -door_half, -sink - 0.1], cube([lobby + 0.1, 2 * door_half, door_h + sink + 0.1]))
    m.add(block - void, house_wall)
    # Lobby: the glass doors at its far end, and a canopy over it.
    m.add(translate([x0 + lobby - 0.02, 0, door_h / 2], cube([0.04, 2 * door_half - 0.2, door_h - 0.1], center=True)),
          house_glass)
    m.add([translate([x0 + lobby - 0.04, y, door_h / 2], cube([0.06, 0.08, door_h], center=True))
           for y in (-door_half / 2, 0, door_half / 2)], [0.2, 0.21, 0.23])
    m.add(translate([x0 - 0.6, 0, door_h + 0.1], cube([1.2, 2 * door_half + 0.8, 0.12], center=True)), house_red)
    # Windows down each side.
    win_z = floor * 0.55
    m.add([translate([(x0 + x1) / 2 + 0.6, s * (half + 0.01), win_z], cube([x1 - x0 - 3.0, 0.04, floor * 0.35], center=True))
           for s in (-1, 1)], house_glass)
    # The platform: a deck over the top, its edge trimmed red.
    m.add(translate([(x0 + x1) / 2, 0, floor + 0.04], cube([x1 - x0, 2 * half, 0.08], center=True)), house_deck)
    trim = [translate([(x0 + x1) / 2, s * (half + 0.03), floor - 0.12], cube([x1 - x0 + 0.06, 0.06, 0.3], center=True))
            for s in (-1, 1)]
    trim += [translate([x, 0, floor - 0.12], cube([0.06, 2 * half + 0.06, 0.3], center=True)) for x in (x0 - 0.03, x1 + 0.03)]
    m.add(trim, house_red)


def add_deck_rails(m, x0, x1, half, floor, h=1.0):
    """Railings down both sides of the platform."""
    y = half - 0.1
    m.add([translate([(x0 + x1) / 2, s * y, floor + h], cube([x1 - x0 - 0.4, 0.06, 0.06], center=True)) for s in (-1, 1)],
          "Silver")
    n = max(int((x1 - x0) / 2.0), 1)
    m.add([translate([x0 + 0.2 + (x1 - x0 - 0.4) * k / n, s * y, floor + h / 2], cube([0.05, 0.05, h], center=True))
           for s in (-1, 1) for k in range(n + 1)], "Silver")


def cabin_house_dims(gap, sweep):
    """A cabin lift's station house's sizes from its cables' gap and how
    far its cabins reach from the axis: the columns just outside that, the
    house and roof out past them, the back behind the wheel's rim, and the
    door's half width round the riders' cable. Mirrored in the world's
    station geometry (gondolaStation, funitelStation, triStation)."""
    col_y = sweep + 0.45
    return dict(col_y=col_y, house_half=col_y + 0.95, roof_half=col_y + 0.9,
                x0=min(-3.4, -gap - 1.9), x1=12.6, door_half=gap + 1.1)


def add_cabin_house(m, gap, sweep, floor, cable_h, cabin_across, ropes=(0.0,), load_x=4.5):
    """A detachable cabin lift's station house: the cabins come off the
    cable and run slow round the bullwheel (gap the radius) on a rail hung
    from a hall roof on six columns, docking floor up on the platform over
    the ground floor, their cable cable_h over it. ropes are each cable's
    ropes across it (the funitel's pair), each with its rail."""
    from build123d import Box, fillet

    d = cabin_house_dims(gap, sweep)
    x0, x1, col_y = d["x0"], d["x1"], d["col_y"]
    roof_bottom = floor + cable_h + 1.35
    rail_z0, rail_z1 = floor + cable_h + 0.15, floor + cable_h + 0.5
    rail_front = 11.8
    steel = [0.36, 0.39, 0.43]
    dark = [0.20, 0.21, 0.23]

    add_station_house(m, x0, x1, d["house_half"], floor, d["door_half"])
    add_housing(m, x0, x1, d["roof_half"], bottom=roof_bottom, wall=roof_bottom + 0.45, rise=0.9 + 0.1 * gap,
                band_color=house_red, slots=False)

    # Rails: a racetrack round the wheel and on along both sides, where the
    # cabins run slow off the cable, one under each rope.
    w = 0.22
    rails = []
    for r in ropes:
        rails += [translate([0, s * (gap + r) - w / 2, rail_z0], cube([rail_front, w, rail_z1 - rail_z0])) for s in (-1, 1)]
        rr = gap + r
        ring = difference(cylinder(h=rail_z1 - rail_z0, r=rr + w / 2, fn=28),
                          translate([0, 0, -0.1], cylinder(h=1, r=rr - w / 2, fn=28)),
                          translate([0, -rr - 1, -0.1], cube([rr + 1, 2 * rr + 2, 1])))
        rails.append(translate([0, 0, rail_z0], ring))
    m.add(rails, steel)

    # Spine down the middle under the roof, arms out to the rails.
    m.add(translate([-1.6, -0.35, rail_z1], cube([rail_front + 1.6, 0.7, roof_bottom - rail_z1 + 0.05])), dark)
    reach = gap + max(ropes)
    m.add([translate([x - 0.1, -reach, rail_z1 - 0.02], cube([0.2, 2 * reach, 0.18])) for x in (1.5, 4.5, 7.5, 10.5)],
          steel)

    # Bullwheel cover over the wheel, and the drive on it.
    m.add(translate([0, 0, rail_z1 - 0.05], cylinder(h=0.5, r=gap + 0.25, fn=28)), "Silver")
    drive = fillet(Box(2.2, 1.6, 0.5).edges().filter_by(lambda e: e.center().Z > 0.15), 0.12)
    m.add(translate([-0.4, 0, rail_z1 + 0.68], drive), house_red, tolerance=0.03)

    # Columns up from the platform, braced into the roof.
    for x in (x0 + 1.2, 2.0, 11.0):
        for s in (-1, 1):
            y = s * col_y
            m.add(translate([x, y, floor], cylinder(h=roof_bottom - floor + 0.1, r=0.22, fn=12)), steel)
            m.add(rod([x, y, roof_bottom - 1.3], [x, s * (col_y - 1.1), roof_bottom + 0.02], 0.09, fn=8), steel)
            m.add(rod([x, y, roof_bottom - 1.3], [x, s * (col_y + 0.8), roof_bottom + 0.02], 0.09, fn=8), steel)

    # Boarding mats under the cabins' straight runs, and the load lines.
    mat_w = cabin_across + 0.4
    m.add([translate([0.6, s * gap - mat_w / 2, floor + 0.08], cube([rail_front - 0.8, mat_w, 0.03])) for s in (-1, 1)],
          [0.30, 0.31, 0.33])
    add_load_lines(m, load_x, length=cabin_across + 0.6, gap=gap, z=floor + 0.1)
    add_deck_rails(m, x0, x1, d["house_half"], floor)

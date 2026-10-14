"""Chair kit — the chairlift chair shared by the double, triple, quad and
6-pack (chair.py, chair_triple.py, chair_quad.py, chair_6pack.py), which
differ in width, cushion colour, and hanger weight.

    +X = forward (direction the chair travels along the cable)
    +Y = seat width (lateral, perpendicular to cable)
    +Z = up

The origin is the cable-attachment point and the chair hangs in -Z. A grip
clamps the cable; a hanger drops behind the riders to a yoke over the
backrest; side frames carry the seat, armrests, and a restraint bar (shown
raised); a footrest hangs under the riders' feet. Riders are placed by the
slots with their feet on the footrest, which hangs cable_h below the
cable: the cable's height at a station (world.BullwheelHeight), so at the
load point a rider's skis are on the snow as the chair picks them up or
sets them down. Everything else hangs from the seat height that gives.

Frame members are round tubes (8 sides) with rounded seat cushions.

Conventions, units, and axis docs: see models-src/README.md.
"""

from build123d import Box, Location, fillet

from lib.mogul import *

# ── Shared dimensions (metres) ─────────────────────────────────────────
cable_h = 3.65  # world.BullwheelHeight
chair_depth = 0.66  # seat front to back (X)
foot_top_z = -cable_h  # footrest top: where riders' feet go (the slots)
seat_top_z = foot_top_z + 0.50  # top of the seat cushion
seat_cushion = 0.12
seat_frame_z = seat_top_z - seat_cushion - 0.04  # bar under the cushion
back_x = -chair_depth / 2  # back edge of the seat
back_bot_z = seat_top_z + 0.02
back_top_z = seat_top_z + 0.38
back_cushion = 0.08
yoke_z = seat_top_z + 0.44  # the bar over the backrest
restraint_z = seat_top_z + 0.78  # the raised restraint bar, clear of riders' heads
grip_len = 0.36
foot_x = 0  # under the riders, at the slots' X
bar = 0.06  # frame tube section
frame_color = [0.24, 0.26, 0.29]
grip_color = [0.55, 0.57, 0.60]


def member(a, b, s=bar):
    """A frame tube from a to b, s across."""
    return rod(a, b, s / 2, fn=8)


def cushion_box(x, y, z, r=0.03):
    """A box with its long edges rounded, centred on the origin."""
    b = Box(x, y, z)
    return fillet(b.edges().filter_by(lambda e: abs(e.tangent_at(0).Y) > 0.9), r)


def chair(m, w, cushion, hanger=0.10):
    """Add a chair w wide to model m: cushion is the seat and backrest
    colour, hanger the hanger tube's section."""
    half = w / 2
    ex = half + 0.03  # side frames, just outside the seat

    # Grip on the cable.
    m.add(translate([0, 0, -0.02], cube([grip_len, 0.16, 0.20], center=True)), grip_color)

    f = []
    # Hanger: down from the grip, then back over the riders' heads to the yoke.
    f.append(member([0, 0, -0.08], [0, 0, yoke_z + 0.18], hanger))
    f.append(member([0, 0, yoke_z + 0.18], [back_x - 0.03, 0, yoke_z], hanger))
    # Yoke over the backrest, the full width.
    f.append(member([back_x - 0.03, -ex, yoke_z], [back_x - 0.03, ex, yoke_z]))
    for s in (-1, 1):
        y = s * ex
        # Side frame: down the back, then forward under the seat.
        f.append(member([back_x - 0.03, y, yoke_z], [back_x - 0.03, y, seat_frame_z]))
        f.append(member([back_x - 0.03, y, seat_frame_z], [-back_x, y, seat_frame_z]))
        # Armrest with a post at its front.
        f.append(member([back_x - 0.03, y, seat_top_z + 0.24], [0.14, y, seat_top_z + 0.24]))
        f.append(member([0.14, y, seat_top_z + 0.24], [0.14, y, seat_frame_z]))
        # Restraint bar arm, raised: from the yoke up and forward.
        f.append(member([back_x - 0.03, y, yoke_z], [0.30, y, restraint_z], 0.05))
        # Footrest hanger from the front of the seat.
        f.append(member([-back_x - 0.02, s * (half - 0.08), seat_frame_z],
                        [foot_x + 0.05, s * (half - 0.08), foot_top_z - 0.02], 0.04))
    # Restraint bar across the front, raised clear of the riders.
    f.append(member([0.30, -ex, restraint_z], [0.30, ex, restraint_z], 0.05))
    # Cross tube under the front of the seat.
    f.append(member([-back_x, -ex, seat_frame_z], [-back_x, ex, seat_frame_z]))
    # Footrest: the riders stand their skis on it.
    f.append(translate([foot_x, 0, foot_top_z - 0.025], cube([0.12, w - 0.1, 0.05], center=True)))
    m.add(f, frame_color)

    # Seat cushion.
    m.add(translate([0, 0, seat_top_z - seat_cushion / 2], cushion_box(chair_depth, w, seat_cushion)),
          cushion, tolerance=0.02)
    # Backrest cushion, leaning back a touch.
    back = translate([0, 0, (back_top_z - back_bot_z) / 2],
                     cushion_box(back_cushion, w, back_top_z - back_bot_z, r=0.025))
    m.add(translate([back_x + back_cushion / 2, 0, back_bot_z], rotate([0, -8, 0], back)),
          cushion, tolerance=0.02)


def bench(m, w, cushion, hanger=0.10):
    """A chair w wide (chair) with a rider's slot every 0.6 m across it,
    feet on the footrest."""
    chair(m, w, cushion, hanger)
    n = round(w / 0.6)
    for i in range(n):
        m.slot(i, 0, (i - (n - 1) / 2) * 0.6, foot_top_z)


# The bubble cover (the 6-pack's and 8-pack's upgrade): a tinted hood
# hinged at the bend of the hanger over the backrest, as wide as the
# chair, with side panels. Down on the line it closes over the riders to
# their knees; up in the terminals it swings back over the top. Its frame
# is part of the chair (chair_*_bubble*.py); the hood's tinted shell is
# its own model (chair_*_hood*.py), which the game draws see-through.
bubble_pivot = (back_x - 0.03, yoke_z + 0.18)
bubble_r = 1.08


def bubble(m, w, color=None, down=True, hood=True, frame=True):
    """Add the bubble cover to a chair w wide, down over the riders or
    up: its tinted hood (coloured color) and its frame."""
    a0, a1 = (-38, 100) if down else (75, 210)
    px, pz = bubble_pivot
    r0, r1 = bubble_r - 0.04, bubble_r
    steps = 10
    arc = [math.radians(a0 + (a1 - a0) * k / steps) for k in range(steps + 1)]
    shell = [(r1 * math.cos(a), r1 * math.sin(a)) for a in arc] + [(r0 * math.cos(a), r0 * math.sin(a)) for a in reversed(arc)]
    pie = [(0.0, 0.0)] + [(r1 * math.cos(a), r1 * math.sin(a)) for a in arc]
    hw = w / 2 + 0.08
    if hood:
        parts = [rotate([90, 0, 0], linear_extrude(shell, height=2 * hw, center=True))]
        parts += [translate([0, s * hw, 0], rotate([90, 0, 0], linear_extrude(pie, height=0.03, center=True)))
                  for s in (-1, 1)]
        m.add([translate([px, 0, pz], p) for p in parts], color)
    if not frame:
        return
    # Its frame: a rim along the free edge, ribs down each end and between
    # the panes, dark against the tinted shell.
    edge = math.radians(a0)
    ex, ez = px + (r1 + 0.02) * math.cos(edge), pz + (r1 + 0.02) * math.sin(edge)
    frame = [translate([ex, 0, ez], rotate([90, 0, 0], cylinder(h=2 * hw, r=0.035, fn=6, center=True)))]
    for y in [-hw, hw] + [hw * (2 * k / 4 - 1) for k in range(1, 4)]:
        rib = [(px + (r1 + 0.015) * math.cos(a), y, pz + (r1 + 0.015) * math.sin(a)) for a in arc]
        frame += [rod(rib[k], rib[k + 1], 0.025, fn=6) for k in range(len(rib) - 1)]
    m.add(frame, frame_color)
    # The hinge bar along the pivot.
    m.add(translate([px, 0, pz], rotate([90, 0, 0], cylinder(h=2 * hw + 0.1, r=0.04, fn=8, center=True))), frame_color)

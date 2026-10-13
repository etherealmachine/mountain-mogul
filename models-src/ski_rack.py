"""Ski rack — a wooden A-frame rack for ten pairs of skis, set on the snow
by a lodge door.

A top rail at shoulder height on an A-frame at each end, and a slotted
foot rail low down in front (−Y) where the tails stand; skis lean back
against the top rail. The game stands each pair in front of the rail
(world.Building.SkiRackSlot: 0.3 m apart along X, 0.3 m out on the front
side). The rails' long edges are rounded like planed timber.

Conventions, units, and axis docs: see models-src/README.md.
"""

from build123d import Box, fillet

from lib.mogul import *

len_x = 3.2  # rail length
rail_h = 1.15  # top rail height
rail_w = 0.09  # rail cross-section
leg_w = 0.08
leg_foot = 0.45  # how far each A-frame leg spreads from the centre line
foot_h = 0.22  # foot rail height
foot_y = 0.30  # foot rail distance in front of the top rail
slot_n = 10
slot_pitch = 0.3

wood = [0.55, 0.38, 0.22]
dark = [0.32, 0.22, 0.14]


def timber(x, y, z, r, axis="X"):
    """A centred box with its edges along axis rounded to r."""
    b = Box(x, y, z)
    v = {"X": lambda t: abs(t.X), "Y": lambda t: abs(t.Y), "Z": lambda t: abs(t.Z)}[axis]
    return fillet(b.edges().filter_by(lambda e: v(e.tangent_at(0)) > 0.9), r)


def leg(x, y):
    """One leg of an A-frame from the ground at (x, y) up to the rail."""
    return hull(translate([x, y, 0], cube([leg_w, leg_w, 0.01], center=True)),
                translate([x, 0, rail_h], cube([leg_w, leg_w, 0.01], center=True)))


m = Model()
m.footprint(len_x / 2 + 0.1, 0.55)

w = []
# Top rail.
w.append(translate([0, 0, rail_h], timber(len_x, rail_w, rail_w, 0.025)))
# A-frames at both ends.
for x in (-len_x / 2 + 0.1, len_x / 2 - 0.1):
    w.append(leg(x, -leg_foot))
    w.append(leg(x, leg_foot))
    # Cross brace.
    w.append(translate([x, 0, 0.5], timber(leg_w, leg_foot * 1.1, 0.06, 0.015, "Y")))
# Foot rail in front, on short posts.
w.append(translate([0, -foot_y, foot_h], timber(len_x - 0.2, 0.12, 0.06, 0.02)))
for x in (-len_x / 2 + 0.1, len_x / 2 - 0.1):
    w.append(translate([x, -foot_y, foot_h / 2], cube([leg_w, leg_w, foot_h], center=True)))
m.add(w, wood, tolerance=0.005)

# Slot dividers on the foot rail, one either side of each pair.
m.add([translate([(i - slot_n / 2) * slot_pitch, -foot_y, foot_h + 0.06], cube([0.03, 0.12, 0.08], center=True))
       for i in range(slot_n + 1)], dark)
m.main()

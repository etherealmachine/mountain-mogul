"""Portal kit — a tunnel mouth (tunnel_portal_rail.py, _road.py,
_path.py): a stone face around an arched opening, dark inside, with
wing walls, set where a line goes into the hill.

    +X = into the tunnel (the face is at x = 0, the hill behind it)
    +Y = across, +Z = up from the line's level

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

stone = [0.55, 0.53, 0.50]
dark_stone = [0.40, 0.39, 0.37]
inside = [0.03, 0.03, 0.035]


def portal(m, w, h, t=1.2):
    """A portal for an opening w wide and h tall (to the top of its arch)."""
    r = w / 2
    spring = h - r  # where the arch starts
    face_w, face_h = w + 5.0, h + 2.5
    opening = union(translate([-1, -r, 0], cube([t + 2, w, spring])),
                    translate([-1, 0, spring], rotate([0, 90, 0], cylinder(h=t + 2, r=r, fn=24))))
    m.add(difference(translate([0, -face_w / 2, -0.5], cube([t, face_w, face_h + 0.5])), opening), stone)
    # The arch's ring of voussoirs, a little proud.
    ring = difference(translate([-0.15, 0, spring], rotate([0, 90, 0], cylinder(h=0.2, r=r + 0.6, fn=24))),
                      translate([-1, 0, spring], rotate([0, 90, 0], cylinder(h=3, r=r, fn=24))),
                      translate([-1, -r - 1, spring - r - 1], cube([3, w + 2, r + 1])))
    m.add(ring, dark_stone)
    # Coping along the top, and wing walls angled back into the slope.
    m.add(translate([-0.2, -face_w / 2 - 0.2, face_h - 0.1], cube([t + 0.4, face_w + 0.4, 0.35])), dark_stone)
    for s in (-1, 1):
        m.add(hull(translate([0, s * face_w / 2 - 0.4, -0.5], cube([t, 0.8, face_h])),
                   translate([-4.0, s * (face_w / 2 + 2.5) - 0.4, -0.5], cube([0.8, 0.8, 1.2]))), stone)
    # Stone over the opening, back past the dark inside, so from above
    # the hill (not drawn just behind the face across the opening:
    # render/tunnel_mouths.go) meets stone.
    m.add(translate([t, -r - 0.6, h], cube([1.4, w + 1.2, face_h - h])), stone)
    # The dark inside: a back wall just past the opening, so it reads
    # as a hole into the hill.
    m.add(translate([t + 0.6, -r, 0], cube([0.2, w, h])), inside)
    m.add(translate([t, -r, 0], cube([0.6, 0.05, spring])), inside)
    m.add(translate([t, r - 0.05, 0], cube([0.6, 0.05, spring])), inside)
    m.add(translate([t, -r, h - 0.05], cube([0.6, w, 0.05])), inside)

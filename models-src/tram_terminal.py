"""Aerial tram terminal — a tram's bottom / top station house: both cabins'
docks on a platform up over an enclosed ground floor
(lib/station_kit.add_station_house), under a tall hall roof on six
columns with the drive house hung in its back end.

The anchor (the "bullwheel" in the game's station frame) is the origin;
+X runs toward the other end. Each cabin's track runs world.TramGap
either side of the axis, world.TramDockHeight over the platform, from
the drive house out through the front of the hall on two ropes
(world.TramRopeSpread); the cabins dock load_x in front of the anchor
(world.StationGeom.LoadAhead), their floors just over the platform.
Riders board from the platform between the docks, where a red line
marks the spot; the line files in under it through the lobby in the
back wall, and riders getting off walk out the same way. The operator
works inside. Keep the footprint in step with world.tramStation.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.station_kit import *

# ── Dimensions (metres) ────────────────────────────────────────────────
floor = 3.0  # world.TramPlatform: the platform over the ground
track_y = 3.2  # world.TramGap
rope_y = 0.4  # world.TramRopeSpread
track_z = floor + 7.0  # world.TramDockHeight over the platform
load_x = 7.0  # world.StationGeom.LoadAhead
cabin_w = 3.5  # tram_cabin.py
cabin_l = 8.6
x0, x1 = -5.2, 14.2  # the house and the roof, back to front
house_half = 6.5
door_half = 2.2  # world.tramStation's DoorHalf
roof_half = 6.3
roof_bottom = floor + 9.6
roof_wall = floor + 10.3
roof_rise = 1.4
col_y = 5.6
col_xs = (-4.0, 4.5, 13.0)
house_x0, house_x1 = -5.0, -0.6  # the drive house, hung under the roof's back end
house_z0 = floor + 6.0

steel = [0.36, 0.39, 0.43]
dark = [0.20, 0.21, 0.23]
red = [0.74, 0.11, 0.13]
yellow = [0.92, 0.74, 0.10]
platform = [0.30, 0.31, 0.33]

m = Model()

add_station_house(m, x0, x1, house_half, floor, door_half, door_h=2.6, lobby=3.0)

# Hall roof, no slots: the cabins hang well under it.
add_housing(m, x0, x1, roof_half, bottom=roof_bottom, wall=roof_wall, rise=roof_rise, band_color=red, slots=False)

# Columns: round steel posts outside the docks, up from the platform,
# braced into the roof.
for x in col_xs:
    for s in (-1, 1):
        y = s * col_y
        m.add(translate([x, y, floor], cylinder(h=roof_bottom - floor + 0.1, r=0.32, fn=12)), steel)
        m.add(rod([x, y, roof_bottom - 1.8], [x, s * (col_y - 1.5), roof_bottom + 0.02], 0.12, fn=8), steel)
        m.add(rod([x, y, roof_bottom - 1.8], [x, s * (col_y + 0.6), roof_bottom + 0.02], 0.12, fn=8), steel)

# Drive house: hung under the back of the roof, clear over the platform,
# with a band of windows and the red trim, the track ropes anchored in
# its front face.
house = translate([(house_x0 + house_x1) / 2, 0, (house_z0 + roof_bottom) / 2],
                  cube([house_x1 - house_x0, 2 * col_y - 1.2, roof_bottom - house_z0 + 0.05], center=True))
m.add(house, "Gainsboro")
m.add(translate([house_x1 + 0.01, 0, house_z0 + 1.9], cube([0.04, 2 * col_y - 2.0, 0.9], center=True)), "LightSteelBlue")
m.add(translate([(house_x0 + house_x1) / 2, 0, house_z0 + 0.12],
                cube([house_x1 - house_x0 + 0.06, 2 * col_y - 1.14, 0.24], center=True)), red)
m.add([translate([house_x1 + 0.25, s * track_y + r, track_z], cube([0.5, 0.5, 0.6], center=True))
       for s in (-1, 1) for r in (-rope_y, rope_y)], dark)

# The track beams the carriages run onto, hung from the roof, one over
# each dock.
for s in (-1, 1):
    y = s * track_y
    m.add(translate([(house_x1 + x1) / 2, y, track_z + 0.55], cube([x1 - house_x1, 2 * rope_y + 0.4, 0.4], center=True)), steel)
    m.add([translate([x, y, (track_z + 0.75 + roof_bottom) / 2], cube([0.2, 0.2, roof_bottom - track_z - 0.7], center=True))
           for x in (1.5, 5.5, 9.5, 13.5)], steel)

# On the platform: the boarding platform between the docks, its edges
# marked yellow, the red line where riders board, and a dark pad at each
# dock.
plat_w = 2 * track_y - cabin_w - 0.1
z = floor + 0.08
m.add(translate([(0.0 + x1) / 2, 0, z + 0.01], cube([x1, plat_w, 0.02], center=True)), platform)
m.add([translate([(0.0 + x1) / 2, s * (plat_w / 2 - 0.08), z + 0.025], cube([x1, 0.12, 0.02], center=True)) for s in (-1, 1)],
      yellow)
m.add(translate([load_x, 0, z + 0.03], cube([0.16, plat_w - 0.3, 0.02], center=True)), "Crimson")
m.add([translate([load_x, s * track_y, z + 0.01], cube([cabin_l + 0.6, cabin_w + 0.3, 0.02], center=True)) for s in (-1, 1)],
      dark)
add_deck_rails(m, x0, x1, house_half, floor)
m.main()

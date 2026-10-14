"""T-bar hanger — a T-bar's hanger let down to its riders: the spring box
on the cable (the origin), its line, the shank and the T, the crossbar
behind two riders' thighs. Drawn as each loaded place on a T-bar's loop
(world.MeshTBarHanger); an empty one is tbar_hanger_up.py. The slots are
the riders' feet on the snow, either side of the shank and a little
ahead of the bar.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.tbar_kit import CABLE_H, BAR_Z, RIDER_Y, add_grip, add_bar

bar_z = BAR_Z - CABLE_H

m = Model()
add_grip(m)
m.add(translate([0, 0, (-0.6 + bar_z + 1.6) / 2], cube([0.02, 0.02, -0.6 - bar_z - 1.6], center=True)), "DimGray")
add_bar(m, bar_z)
m.slot(0, 0.25, RIDER_Y, -CABLE_H)
m.slot(1, 0.25, -RIDER_Y, -CABLE_H)
m.main()

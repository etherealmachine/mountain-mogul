"""T-bar hanger, up — an empty T-bar hanger, its line reeled into the
spring box so the shank and T hang just under the cable, out of the way.
Drawn for each empty place on a T-bar's loop; a loaded one is
tbar_hanger.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.tbar_kit import add_grip, add_bar

m = Model()
add_grip(m)
add_bar(m, -1.15, shank=0.55)
m.main()

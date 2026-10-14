"""T-bar base — the bottom end of a T-bar: the bullwheel flat at cable
height on a boom off a mast, the drive motor on the mast, and the
operator's control post. Riders take a bar on -Y. The top end is
tbar_top.py; the parts are in lib/tbar_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.tbar_kit import add_tbar_end

m = Model()
add_tbar_end(m, -1, base=True)
m.main()

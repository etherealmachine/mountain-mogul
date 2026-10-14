"""Rope tow top — the top end of a rope tow: the bullwheel on its frame and
a stop post. Placed facing back down the line, so its riders, on the same
side of the line all the way up, are on +Y here. The parts are in
lib/rope_tow_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rope_tow_kit import add_rope_end

m = Model()
add_rope_end(m, +1, base=False)
m.main()

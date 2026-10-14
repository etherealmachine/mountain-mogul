"""Rope tow base — the bottom end of a rope tow: the bullwheel on its frame,
the motor that drives it, and the operator's control post. Riders take
hold of the rope on -Y. The top end is rope_tow_top.py; the parts are in
lib/rope_tow_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rope_tow_kit import add_rope_end

m = Model()
add_rope_end(m, -1, base=True)
m.main()

"""T-bar top — the top end of a T-bar: the bullwheel on its boom and mast,
the tension weight on the mast, and the top operator's control post.
Placed facing back down the line, so its riders are on +Y. The parts are
in lib/tbar_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.tbar_kit import add_tbar_end

m = Model()
add_tbar_end(m, +1, base=False)
m.main()

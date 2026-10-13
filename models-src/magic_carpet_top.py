"""Magic carpet top — the top end of a magic carpet: magic_carpet_end.py
mirrored. The end is placed facing back down the line, so the belt, on the
same side of the line all the way up, lies on +Y here. Riders glide
straight off over the plate. The parts are in lib/carpet_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.carpet_kit import add_carpet_end

m = Model()
add_carpet_end(m, +1)
m.main()

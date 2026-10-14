"""Bubble hood — the 6-pack's tinted bubble cover down over the riders, as on the line
(lib/chair_kit.bubble): the shell and side panels only, which the game
draws see-through over chair_6pack_bubble.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bubble
from lib.mogul import Model

m = Model()
bubble(m, 3.60, [0.55, 0.75, 0.95], down=True, frame=False)
m.main()

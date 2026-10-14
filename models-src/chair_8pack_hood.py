"""Bubble hood — the 8-pack's tinted bubble cover down over the riders, as on the line
(lib/chair_kit.bubble): the shell and side panels only, which the game
draws see-through over chair_8pack_bubble.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bubble
from lib.mogul import Model

m = Model()
bubble(m, 4.80, [1.00, 0.62, 0.22], down=True, frame=False)
m.main()

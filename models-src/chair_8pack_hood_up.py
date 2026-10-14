"""Bubble hood — the 8-pack's tinted bubble cover up, as in the terminals
(lib/chair_kit.bubble): the shell and side panels only, which the game
draws see-through over chair_8pack_bubble_up.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.chair_kit import bubble
from lib.mogul import Model

m = Model()
bubble(m, 4.80, [1.00, 0.62, 0.22], down=False, frame=False)
m.main()

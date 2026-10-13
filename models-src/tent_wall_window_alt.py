"""Tent shell kit: wall window alt. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").wall_window_alt(m)
m.main()

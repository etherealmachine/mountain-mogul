"""Lodge shell kit: wall with a shuttered window. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").wall_window_alt(m)
m.main()

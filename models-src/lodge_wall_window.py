"""Lodge shell kit: wall with a window. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").wall_window(m)
m.main()

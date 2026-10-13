"""Lodge shell kit: plain wall. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").wall_panel(m)
m.main()

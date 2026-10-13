"""Lodge shell kit: hip, peaking at the (−X, +Y) corner. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").roof_tile(m, [1, 0, 0, 0], 0)
m.main()

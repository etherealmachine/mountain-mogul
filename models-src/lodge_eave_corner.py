"""Lodge shell kit: eaves past an outer corner. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").eave_corner(m)
m.main()

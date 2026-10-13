"""Lodge shell kit: eaves strip. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").eave(m)
m.main()

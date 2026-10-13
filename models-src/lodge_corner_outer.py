"""Lodge shell kit: outer corner post. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").corner_outer(m)
m.main()

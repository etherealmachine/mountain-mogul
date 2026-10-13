"""Lodge shell kit: inner corner trim. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").corner_inner(m)
m.main()

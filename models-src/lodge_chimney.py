"""Lodge shell kit: stone chimney. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").chimney(m)
m.main()

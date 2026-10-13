"""Lodge shell kit: glazed wall (food courts). See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("lodge").wall_glazed(m)
m.main()

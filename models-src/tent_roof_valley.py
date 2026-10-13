"""Tent shell kit: roof valley. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").roof_tile(m, [1, 1, 0, 1], 0)
m.main()

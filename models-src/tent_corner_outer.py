"""Tent shell kit: corner outer. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").corner_outer(m)
m.main()

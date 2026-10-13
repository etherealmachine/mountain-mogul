"""Tent shell kit: corner inner. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").corner_inner(m)
m.main()

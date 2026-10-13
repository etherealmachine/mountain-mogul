"""Tent shell kit: door. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").door(m)
m.main()

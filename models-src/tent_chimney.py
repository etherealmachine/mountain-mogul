"""Tent shell kit: chimney. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").chimney(m)
m.main()

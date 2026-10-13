"""Tent shell kit: wall glazed. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("tent").wall_glazed(m)
m.main()

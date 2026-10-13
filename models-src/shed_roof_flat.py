"""Shed shell kit: roof flat. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("shed").roof_tile(m, [0, 0, 0, 0], 0)
m.main()

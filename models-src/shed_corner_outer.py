"""Shed shell kit: corner outer. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("shed").corner_outer(m)
m.main()

"""Shed shell kit: chimney. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("shed").chimney(m)
m.main()

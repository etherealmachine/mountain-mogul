"""Shed shell kit: eave corner. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("shed").eave_corner(m)
m.main()

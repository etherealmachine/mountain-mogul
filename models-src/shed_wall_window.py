"""Shed shell kit: wall window. See lib/lodge_kit.py."""

from lib.lodge_kit import LodgeKit
from lib.mogul import Model

m = Model()
LodgeKit("shed").wall_window(m)
m.main()

"""Tunnel portal for a road — an opening 9.0 m wide and 6.0 m tall
(lib/portal_kit.py).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.portal_kit import portal

m = Model()
portal(m, 9.0, 6.0)
m.main()

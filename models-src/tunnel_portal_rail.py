"""Tunnel portal for a rail — an opening 5.0 m wide and 6.8 m tall
(lib/portal_kit.py).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.portal_kit import portal

m = Model()
portal(m, 5.0, 6.8)
m.main()

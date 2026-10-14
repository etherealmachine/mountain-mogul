"""Tunnel portal for a path — an opening 3.2 m wide and 3.2 m tall
(lib/portal_kit.py).

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.portal_kit import portal

m = Model()
portal(m, 3.2, 3.2)
m.main()

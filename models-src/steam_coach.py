"""Steam coach — an old wooden coach for the steam train (rail_kit.py):
a maroon body with cream window panels, a clerestory roof, open end
platforms with railings, two bogies. 12.4 m over the buffers.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

maroon = [0.42, 0.11, 0.09]
cream = [0.86, 0.80, 0.62]
roof = [0.30, 0.30, 0.32]

m = Model()
bogie(m, 3.8, base=2.0, r=0.45)
bogie(m, -3.8, base=2.0, r=0.45)
m.add(box(-6.0, 6.0, 2.5, 0.95, 1.15), dark)  # underframe
# Body: maroon below and between windows, cream panels round them.
m.add(body(-5.4, 5.4, 2.7, 1.15, 3.2, r=0.08), maroon)
stripe(m, -5.3, 5.3, 2.7, 2.15, 2.95, cream)
window_band(m, -5.0, 5.0, 2.72, 2.25, 2.85, gap=0.25, pane=0.75)
# Roof and clerestory.
m.add(section(roof_profile(2.85, 3.2, 0.22), -5.55, 5.55), roof)
m.add(box(-4.2, 4.2, 1.1, 3.35, 3.65), roof)
window_band(m, -4.0, 4.0, 1.12, 3.43, 3.57, gap=0.3, pane=0.5, color=[0.85, 0.82, 0.55])
# End platforms with railings, and buffers.
for e in (-1, 1):
    m.add(box(min(e * 5.4, e * 6.0), max(e * 5.4, e * 6.0), 2.5, 1.1, 1.2), [0.35, 0.22, 0.12])
    for s in (-1, 1):
        m.add(rod([e * 5.95, s * 1.2, 1.2], [e * 5.95, s * 1.2, 2.2], 0.03, fn=6), dark)
        m.add(rod([e * 5.45, s * 1.2, 2.2], [e * 5.95, s * 1.2, 2.2], 0.03, fn=6), dark)
    m.add(rod([e * 5.95, -1.2, 2.2], [e * 5.95, -0.35, 2.2], 0.03, fn=6), dark)
    m.add(rod([e * 5.95, 0.35, 2.2], [e * 5.95, 1.2, 2.2], 0.03, fn=6), dark)
    buffers(m, e * 6.0, e)
m.main()

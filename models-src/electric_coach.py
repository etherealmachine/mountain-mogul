"""Double-deck coach — a German regional double-decker (rail_kit.py),
after DB's Dosto: a traffic-red body with two rows of windows, its
lower deck dropped between the bogies, light grey roof. 26.8 m over the
buffers.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.rail_kit import *

red = [0.78, 0.08, 0.10]
light = [0.80, 0.81, 0.82]

m = Model()
for x in (9.6, -9.6):
    bogie(m, x, base=2.5, r=0.46)
# Ends over the bogies sit high; the middle drops low between them.
m.add(body(-13.3, 13.3, 2.8, 1.3, 4.35, r=0.3), red)
m.add(body(-7.8, 7.8, 2.8, 0.55, 1.3, r=0.1), red)
m.add(section(roof_profile(2.7, 4.35, 0.28), -13.2, 13.2), light)
# Lower and upper deck windows; a white band between.
window_band(m, -7.4, 7.4, 2.8, 1.05, 2.05, gap=0.3, pane=1.55)
window_band(m, -12.6, 12.6, 2.8, 2.7, 3.75, gap=0.25, pane=1.7)
stripe(m, -13.2, 13.2, 2.8, 2.2, 2.5, [0.92, 0.92, 0.92])
# Doors near each end.
for e in (-1, 1):
    m.add(box(e * 9.0 - 0.65, e * 9.0 + 0.65, 2.84, 1.35, 3.4), [0.62, 0.64, 0.66])
    buffers(m, e * 13.4, e)
m.main()

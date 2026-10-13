"""Magic carpet end — the base end of a magic carpet: a low ramped end
plate where the belt turns under (the "bullwheel" of world.Lift's loop),
and the post with the stop/start buttons. No booth: the operator stands on
the snow beside the post. The top end is magic_carpet_top.py, the same
mirrored.

The origin is the belt's end, +X runs up the belt toward the other end.
The belt itself is drawn by the game (render.generateBeltMesh), on the
riders' side only: at the base riders stand on -Y (the game's +Z after
export), 1.5 m off the axis (world.CableGap), and the belt's return run is
under it. Riders step on just past the plate (world.StationGeom.LoadAhead).
The parts are in lib/carpet_kit.py.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *
from lib.carpet_kit import add_carpet_end

m = Model()
add_carpet_end(m, -1)
m.main()

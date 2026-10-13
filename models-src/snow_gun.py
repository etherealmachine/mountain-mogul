"""Snow gun — industrial snowmaking cannon on a tripod mount.

A narrow tripod base spreads three legs outward around a central hub. A
vertical swivel post rises from the hub, and a horizontal cannon barrel
points along +X (the direction the nozzle faces). The game spreads snow
in a uniform circle regardless of barrel orientation, but the directional
silhouette reads clearly at resort scale.

Real snow guns are 0.5–1 m in diameter and 1–2 m long; this model is
sized to be visible at the same zoom level as a patrol hut without
dominating the terrain.

The OpenSCAD source faceted everything at $fn = 12; the Python port draws
the hub, post, barrel and nozzle as true curves.

Conventions, units, and axis docs: see models-src/README.md.
"""

from lib.mogul import *

# ── Dimensions (metres) ────────────────────────────────────────────────
# Tripod
leg_len = 0.9  # leg length from hub centre
leg_w = 0.08  # leg cross-section (square)
hub_r = 0.22  # hub cylinder radius
hub_h = 0.35  # hub cylinder height

# Swivel post
post_r = 0.12
post_h = 0.75

# Cannon barrel (horizontal, angled slightly up)
barrel_r = 0.11
barrel_len = 1.1
barrel_tilt = 12  # degrees above horizontal

# Nozzle shroud (cone at the barrel tip)
nozzle_r = 0.22  # outer radius at the open end
nozzle_h = 0.18

base_z = hub_h + post_h

m = Model()

# ── Tripod ─────────────────────────────────────────────────────────────
# Hub cylinder sitting on the ground.
m.add(cylinder(r=hub_r, h=hub_h), "DimGray")
# Three legs — thin rectangular prisms splayed 120° apart around Z.
for angle in (0, 120, 240):
    m.add(rotate([0, 0, angle], translate([leg_len / 2, 0, leg_w / 2], cube([leg_len, leg_w, leg_w], center=True))), "DimGray")

# ── Swivel post ────────────────────────────────────────────────────────
# Rising from the top of the hub.
m.add(translate([0, 0, hub_h], cylinder(r=post_r, h=post_h)), "DimGray")

# ── Barrel ─────────────────────────────────────────────────────────────
# Cylinder on the swivel post, barrel_tilt degrees above horizontal,
# pointing along +X.
m.add(
    translate([0, 0, base_z], rotate([0, -barrel_tilt, 0], translate([barrel_len / 2, 0, 0], rotate([0, 90, 0], cylinder(r=barrel_r, h=barrel_len, center=True))))),
    "SlateGray",
)

# ── Nozzle shroud ──────────────────────────────────────────────────────
# A cone at the +X tip of the barrel suggesting the fan-spray outlet.
tip_x = barrel_len * math.cos(math.radians(barrel_tilt))
tip_z = barrel_len * math.sin(math.radians(barrel_tilt))
m.add(
    translate([tip_x, 0, base_z + tip_z], rotate([0, -barrel_tilt, 0], rotate([0, 90, 0], cylinder(r1=barrel_r, r2=nozzle_r, h=nozzle_h)))),
    "SlateGray",
)

m.main()

// Parking lot — flat asphalt pad with painted stall stripes. Background
// for the dynamic car batch which draws individual vehicles on top
// based on the building's CurrentCars count, so the visible "fullness"
// of the lot scales with skier arrivals/departures.
//
// Stripe pitch is aligned with the renderer's per-car stall pitch (2.5 m
// along +X) so painted lanes line up with where cars actually sit. Car
// rows extend along +Y (a car's long axis is +Y in SCAD = game Z).
//
// Conventions, units, and axis docs: see models-src/README.md.
//
//   +X = stall-width direction (stalls repeat along this axis)
//   +Y = stall-length direction (a car is parked along this axis)
//   +Z = up

$fn = 8;

// ── Dimensions (metres) ────────────────────────────────────────────────
pad_w = 40.0; // along +X — total width of the lot (16 × 2.5 m stalls)
pad_d = 30.0; // along +Y — total depth (≈6 rows × 5 m)
pad_h = 0.60; // graded earthwork pad — thicker than asphalt alone (≈15 cm)
              // because the visible slab also stands in for the built-up
              // dirt base under it. A thin slab reads as a sheet of
              // colour and visually merges with the surrounding apron
              // snow at oblique camera angles.

stall_pitch  = 2.5;  // distance between adjacent stall stripes (matches renderer)
stripe_width = 0.15; // metres — a hair wider than reality so it reads at game scale
stripe_inset = 0.5;  // metres pulled in from the pad's +Y / −Y edges
stripe_z     = pad_h + 0.01; // sits just above the asphalt to dodge z-fighting

// ── Geometry ───────────────────────────────────────────────────────────
module pad() {
    // Dark asphalt slab centred on the origin. Slightly raised so it
    // reads as a built-up earthwork pad rather than melting into the snow.
    color([0.20, 0.20, 0.22])
        translate([0, 0, pad_h / 2])
            cube([pad_w, pad_d, pad_h], center = true);
}

// One white stripe centred at x, running along +Y with a small inset
// from each end so the perimeter looks tidy.
module stripe(x) {
    stripe_len = pad_d - 2 * stripe_inset;
    color([0.92, 0.92, 0.92])
        translate([x, 0, stripe_z])
            cube([stripe_width, stripe_len, 0.02], center = true);
}

// Interior stripes only — between each pair of stalls. n_stalls is the
// number of stall columns; we draw (n_stalls − 1) stripes between them
// and skip the outer perimeter so the lot reads "edged by the pad", not
// "lined like a checkerboard".
module stripes() {
    n_stalls = floor(pad_w / stall_pitch);
    for (i = [1 : n_stalls - 1])
        stripe(-pad_w / 2 + i * stall_pitch);
}

module parking_lot() {
    pad();
    stripes();
}

parking_lot();

// ── Footprint metadata ─────────────────────────────────────────────────
// Half-extents in SCAD coords (X, Y). The scad2obj converter applies the
// SCAD → game rotation and writes the canonical (halfX, halfZ) into the
// OBJ header for the placement pass and the per-car layout.
echo("MOGUL_META", "footprint", pad_w / 2, pad_d / 2);

// ── Driveway slots ─────────────────────────────────────────────────────
// Eight attach points for the road network — two on each of the lot's
// four edges. Each slot sits 1 m past the pad edge so the connecting
// road quad has a small gap before the parking apron starts.
//
// On the short faces (±X) the pair is spaced ±pad_d/4 along Y; on the
// long faces (±Y) the pair is spaced ±pad_w/4 along X. Pair spacing
// is wide enough that two roads converging from different angles
// don't fight for the same attach point.
echo("MOGUL_META", "slot", 0,  pad_w / 2 + 1,  pad_d / 4, 0);
echo("MOGUL_META", "slot", 1,  pad_w / 2 + 1, -pad_d / 4, 0);
echo("MOGUL_META", "slot", 2, -pad_w / 2 - 1,  pad_d / 4, 0);
echo("MOGUL_META", "slot", 3, -pad_w / 2 - 1, -pad_d / 4, 0);
echo("MOGUL_META", "slot", 4,  pad_w / 4,  pad_d / 2 + 1, 0);
echo("MOGUL_META", "slot", 5, -pad_w / 4,  pad_d / 2 + 1, 0);
echo("MOGUL_META", "slot", 6,  pad_w / 4, -pad_d / 2 - 1, 0);
echo("MOGUL_META", "slot", 7, -pad_w / 4, -pad_d / 2 - 1, 0);

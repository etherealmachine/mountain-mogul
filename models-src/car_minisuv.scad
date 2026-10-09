// Mini SUV — a small crossover: short hood, tall hatchback, more ground
// clearance than a sedan. Seats four. Parts and conventions:
// lib/car_kit.scad.
include <lib/car_kit.scad>

vehicle(len = 4.35, w = 1.82, r = 0.35, wb = 2.60, sill = 0.28,
        belt = 0.98, hood = 0.90, nose = 0.14, tailin = 0.06,
        roof = 1.62, gb = [-1.95, 0.95], gr = [-1.80, 0.25]);

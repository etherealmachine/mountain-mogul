// Sedan — a four-door saloon: long hood, glass set back, a trunk.
// Seats four. Parts and conventions: lib/car_kit.scad.
include <lib/car_kit.scad>

vehicle(len = 4.50, w = 1.80, r = 0.32, wb = 2.70, sill = 0.20,
        belt = 0.88, hood = 0.80, nose = 0.12, tailin = 0.10,
        roof = 1.42, gb = [-1.30, 0.75], gr = [-0.85, 0.20], axle_x = 0.05);

// SUV — a full-size sport utility: long and square, high roof with
// rails, big wheels. Seats four (the demand model fills it like a car).
// Parts and conventions: lib/car_kit.scad.
include <lib/car_kit.scad>

vehicle(len = 4.85, w = 1.95, r = 0.40, wb = 2.90, sill = 0.34,
        belt = 1.10, hood = 1.04, nose = 0.12, tailin = 0.04,
        roof = 1.86, gb = [-2.30, 1.00], gr = [-2.25, 0.40], rails = true);

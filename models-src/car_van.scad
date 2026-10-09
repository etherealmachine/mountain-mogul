// Van — a minivan: a short sloping nose and one long box of glass.
// Seats seven, so a family or a carload of friends comes in one van
// (sim.vehicleSeats). Parts and conventions: lib/car_kit.scad.
include <lib/car_kit.scad>

vehicle(len = 4.90, w = 1.95, r = 0.35, wb = 3.00, sill = 0.24,
        belt = 1.02, hood = 0.86, nose = 0.20, tailin = 0.04,
        roof = 1.82, gb = [-2.35, 1.40], gr = [-2.30, 0.55], axle_x = 0.10);

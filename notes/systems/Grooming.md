---
title: Grooming
kind: system
status: shipped
---

# Grooming

Snowcats live at a maintenance shed and work a painted route split into one section per cat. They park while the lifts run. After closing, each active cat makes one pass of its section that night if any snow-covered cell in it has worn below 90% groomed. A groomed cell becomes packed powder with fresh corduroy, and moguls are knocked down.

Each groomed trail is laid out as side-by-side passes 4.5 m apart with a 5 m tiller: down the fall line, or along the trail on narrow runs that cross the slope, like cat tracks. Passes curve with the run and converge in gullies. A cat drives its passes in order, alternating direction, with a U-turn between neighbours and a straight transit between groups. Sections are sets of passes, assigned to sheds by distance and capacity and split among a shed's cats by length. Cats still drive straight over trees between passes.

Gameplay stays on 5 m cells: a cell is groomed when the cat's swath covers its centre. What's drawn comes from a 1 m groom texture the cat stamps along the path it actually drove, so lanes, seams, and U-turn marks follow the route instead of the grid. The terrain shader lights corduroy as fine ridges along each lane (exaggerated to about 25 cm so they read from the normal camera), which catch low sun and go flat at noon. Ridges carry across neighbouring passes, with a faint seam where lanes meet. Skier tracks cut through the corduroy. Its strength is the stamp times cell grooming, so skier wear, snowfall, and avalanches fade it; a snowfall heavy enough to bury the corduroy completely clears the whole stamp. The texture is saved with the game, and older saves restamp it from cell grooming.

[[Skiing]] wears grooming off the lines guests use and builds moguls on ungroomed snow. Guests who like groomed runs score corduroy higher, so grooming feeds [[Satisfaction]]. It changes the [[Snow]] surface but not its depth.

Cats cost a purchase price and a daily cost that differs between active and standby, which shows up in [[Finance]].

Spec: [[Snow Spec]]. Plan: [[Real Grooming]]. Code: `internal/sim/snowcats.go`, `internal/sim/groom_passes.go`, `internal/world/snowcat.go`, `internal/world/groom_map.go`, `assets/shaders/terrain.frag`.

## Log

- 2026-10-01: Sheds, routes, sections, nightly passes, and active and standby costs are in.
- 2026-10-05: [[Real Grooming]]: passes follow the run, cats drive them with U-turns, a saved 1 m groom texture records where they went, and corduroy is lit ridges with seams and turn marks.

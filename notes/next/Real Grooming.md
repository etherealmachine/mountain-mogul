---
title: Real Grooming
kind: plan
status: shipped
---

# Real Grooming

Groomed runs that look like a snowcat made them: lanes that follow the run, corduroy lit by the sun, seams where passes meet, and turn marks at the ends. Priority 2 in [[Next Steps]]. Covers [[Hiding the Grid]] step 3 and the corduroy work in [[Grooming]].

## Where it is today

- **Cats sweep north–south columns.** `planRoute` in `internal/sim/snowcats.go` sweeps each trail one x-column at a time, straight up and down, with a one-cell sideways hop between columns. A cat grooms the whole cell it arrives at. Sections are (trail, x-column) pairs.
- **So stamping the cat's path alone wouldn't help.** Its path is axis-aligned, so lanes would still run north–south whatever way the run points. The route has to change first.
- **Corduroy is a brightness sine** at 4 stripes per metre, laid across the smoothed fall line, scaled by per-cell grooming. The crisp groomed edge is a 1 m band along every cell boundary where grooming steps (`RecomputeGroomEdges`, the surface-detail B channel).
- **Trails are bags of cells** with no direction or centerline. Direction has to come from the terrain.
- Sections aren't saved; they're rebuilt on load.

## Approach

Gameplay stays on cells: friction, satisfaction, and the nightly "needs a pass" check still read per-cell `Grooming`. The cat's route and what's drawn move off the grid.

1. **Passes follow the run.** For each groomed trail, plan passes as world-space lines:
   - Start at the highest trail cell not yet covered and trace up and down the smoothed fall line in 1 m steps, stopping at the trail edge.
   - Mark a 4.5 m swath as covered (a 5 m tiller with 0.5 m overlap), and repeat until about 95% of the trail is covered. This handles curves, gullies, and ridges without a centerline.
   - On narrow trails that cross the slope (cat tracks, traverses), passes run along the trail's length instead of the fall line.
   - Plans are cached per trail and rebuilt when its cells or the terrain change.
2. **Cats drive passes.**
   - A route becomes a line of waypoints: passes chained nearest-first, alternating direction, joined by a U-turn to the neighbouring pass or a straight transit to the next group.
   - Sections become sets of passes, assigned to sheds by the same capacity-weighted distance (from each pass's midpoint) and split among a shed's cats by length.
   - Cells whose centre falls inside the swath get `Grooming = 1` as the cat passes, so gameplay is unchanged.
   - Cats still drive straight over trees between passes, as now.
3. **A groom texture records what the cat did.** A new 1 m texture over the map:
   - R: groomed.
   - G and B: the cat's heading, stored so it blends correctly between pixels.
   - A: seams, where a pass overlaps one from the same night.
   - The cat stamps its swath along the path it actually drove each tick, the way skier tracks are drawn, so it stays continuous at any speed.
   - A storm or avalanche that resets grooming clears the stamp there.
   - The groomed edge comes from this mask in the shader, so `RecomputeGroomEdges` and the surface-detail B channel retire.
   - Lift aprons and pads that set grooming directly keep the plain per-cell look; they're square anyway.
   - Size is small: about 14 MB for Boreal, against about 230 MB for the existing 25 cm surface-detail texture.
4. **Corduroy in the shader.**
   - Ridges run across the stamped heading as a bump in the lighting, so they catch low sun and go flat at noon, instead of a brightness stripe.
   - Seams show as a faint line, and turn marks come for free from the heading swinging through each U-turn.
   - Strength is the stamp times per-cell grooming, so skier wear fades it as now.
   - Skier tracks cut through the corduroy (today they're suppressed on groomed snow).
   - Ridges fade out once they're smaller than a pixel, leaving the groomed tint, like the other snow detail.

## Checking

- **Headless tests:** pass coverage at least 95% of trail cells; no pass strays more than 2.5 m outside its trail; every trail cell groomed after one night; sections balanced by length.
- **Screenshots:** before and after on the "Snowcat U-shaped" testbed, a new testbed with a curving diagonal run and a cross-slope cat track, and a groomed run at [[Boreal]] and [[Kirkwood]]. Low sun (`-clock-hour 8`) for the ridges.
- **GPU time** from `-screenshot` on the same views, so corduroy doesn't bring back the storm-lag cost.

## What shipped

All four steps, with these changes from the plan:

- **Passes are evenly spaced streamlines** (`internal/sim/groom_passes.go`). Each trail cell gets a lane direction: the fall line, or along the trail where it's narrow across the slope. The field is smoothed so lanes bend from a run onto a cat track. New passes are seeded one 4.5 m spacing beside an accepted one and stop when they close on another, so lanes converge in gullies instead of piling up.
- **The groom texture stores a lateral coordinate, not a heading.** Ridges have to line up across neighbouring passes, and a heading can't do that. Each pass point carries a distance across the run, continued from the nearest neighbouring lane, and the texture stores it as cos and sin of a 40 m period. The texture is RGBA16, because 8 bits stair-stepped the ridges. Ridges run along each lane, as a tiller leaves them.
- **A is lane ownership, not seams.** It holds the night and how deep the pixel sits inside its lane, so where two passes overlap, the pixel keeps whichever lane it's deeper inside, and a new night overwrites the last. The shader finds seams where neighbouring pixels disagree about the coordinate.
- **Swaths overlap by 0.5 m** past the tiller, so diverging lanes and lane ends leave no slivers.
- **The shader works out the ridge direction** from the screen-space gradient of the coordinate, so no heading needs storing.
- `RecomputeGroomEdges` and the surface-detail B channel are retired; the groomed edge comes from the stamp. Lift aprons, testbeds, and saves without a groom map stamp whole cells down the fall line (`StampCell`, `RestampGroomFromCells`).
- Debug: `-groom-now` with `-screenshot` grooms every section before capture, and the console command `groom` does it in game. New testbed: "Snowcat curving run and cat track".
- GPU time stayed at about 4–8 ms on the test views.

## Left over

- **Edge artifacts**: some artifacts still show along the edge of the groomed area. Listed under Rendering in [[Next Steps]].
- **Grooming over existing tracks**: decide how a cat pass should treat skier tracks already in the snow (wipe them under the swath, fade them, or leave them for the shader to cut through as now). Listed under Rendering in [[Next Steps]].

## Not in this

- Cats pathfinding around trees.
- Winch cats on steep runs.
- Player-drawn grooming routes.
- Daytime grooming.

## Decisions

- Passes follow the fall line; cross-slope trails run along their length.
- The groom texture is saved with the game, compressed.
- Corduroy stays exaggerated (about 25 cm) so it reads from the normal camera.

## Log

- 2026-10-05: Planned. Found that cats sweep north–south columns, so the route has to follow the run before stamping the path helps.
- 2026-10-05: Approved as planned: fall-line passes, saved groom texture, exaggerated corduroy, all four steps.
- 2026-10-05: Shipped. Switched the texture from heading to a lateral coordinate in RGBA16 so ridges carry across passes, and A to lane ownership so overlaps and new nights resolve cleanly.

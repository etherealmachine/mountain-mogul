---
title: Lot Surfaces
kind: plan
status: planned
---

# Lot Surfaces

Parking lots in asphalt, gravel, or dirt, each with its own cost, capacity, and look, so a small or budget resort can start with a dirt lot and pave it later. Split out of [[Transit]] step 4, which built the lots it applies to; see [[Parking and Roads]].

## Today

- Every lot is asphalt: dark grey with white stall stripes, $100 per m², stalls of 2.5 × 5 m in double-loaded rows with 6 m aisles (`internal/world/parking.go`, `internal/render/parking_mesh.go`).
- The lot and its shoulder are plowed bare; the ring past them is ramped back to natural snow.

## Decisions

Made with the user on 2026-10-06 (in [[Transit]]):

- **Three surfaces: asphalt, gravel, dirt.** They differ in cost, capacity, and look.
- **No spring mud effects.**
- **Later, cars drive more slowly on gravel and dirt.**

## Steps

1. **Surface per lot.** A `Surface` on each lot, saved, asphalt by default; old saves load as asphalt. The lot popup (game and editor) picks it, and the Parking tool draws in the last surface picked.
2. **Cost.** A price per m² for each surface (a starting guess: asphalt $100, gravel $40, dirt $15). Drawing and extending charge the lot's surface. Changing a lot's surface charges the difference to pave up (the full new price over its area) and refunds nothing going down.
3. **Capacity.** Gravel and dirt have no striping, so cars park less neatly: wider stall pitch on gravel (about 2.8 × 5.5 m) and wider still on dirt (about 3.0 × 6.0 m), with the same aisles. The popup and the drag toast show the stall count for the surface.
4. **Look.** Asphalt keeps its stripes. Gravel is a light grey, speckled surface with no stripes; dirt is packed brown with a dusting of packed snow, no stripes. Both keep the rounded corners.
5. **Later.** Slower driving on gravel and dirt (the lot speed scaled down per surface); resurfacing as a seasonal upgrade.

## Open questions

- The prices: the guesses above aren't tuned against [[Finance]] or [[First Week Balance]].
- Whether gravel and dirt hold snow differently: plowed to packed snow rather than bare ground, which would change how they look in midwinter.
- Whether guests care: a dirt lot might cost a little satisfaction at a big resort, or make parking slower to get out of at day's end.

## Log

- 2026-10-06: Written up from [[Transit]] step 4 at the user's request, to build later.

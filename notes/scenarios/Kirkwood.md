---
title: Kirkwood
kind: scenario
status: partial
---

# Kirkwood

South of Lake Tahoe, California. A high, remote valley with a gentle beginner area at one end and steep cornices and chutes above. Step 2 of the [[Scenario Campaign]].

**Premise.** Beginners and experts both show up. Serve each without putting them on the same runs.

**Teaches.** Matching terrain to skill. Beginners need green runs off their own lifts, experts want steep blacks, and mixing them causes falls and bad moods ([[Trails]], [[Skiing]], [[Satisfaction]]). Placing lifts so each group finds its terrain ([[Lifts]]).

**Goals (draft).** A good rating from beginners and from experts separately; a set number of guests of each skill a day.

**Rules.** None. The terrain is the challenge.

**Map.** `kirkwood.save`, playable from New Game as step 2 (difficulty 2). Built from real elevation:

- a 3 km square (600 × 600 cells) centred on 38.676 N, 120.068 W: the meadow in the north, the cirque walls across the middle, the summit ridge and a bit of the backside in the south; 630 m of relief
- forest from the editor's generator at 40% coverage and a 72% treeline (seed 2): trees along the drainages and the lower mountain, open bowls above; about 87,000 trees
- a road in from the north edge (Kirkwood Meadows Drive) to a parking lot at the real village site
- one double chair, the beginner lift: 550 m long with 110 m of rise, from just south of the lot heading southeast
- an owned parcel around the base and the chair, and the rest sold as a 6 × 6 grid of parcels at $50,000 each
- $250,000 cash and the default credit line, starting December 1, 2026; no snow, like [[Boreal]]

It's bigger than Boreal (2.5× the cells, 2.8 MB against 1.4 MB) and draws at about 35 fps where Boreal draws about 42 in the same screenshot test.

**Still needs.**

- Rating broken down by skill, so the goal can be measured
- What each skill wants: beginners rentals and easy terrain, intermediates food and rest, experts no crowds (in [[Next Steps]])
- Guests choosing runs by difficulty, which the trail graph already supports
- Surface lifts for the beginner area (in [[Next Steps]])
- Goals ([[Scenario Goals and Rules]])
- North and south faces that look and ski very differently, so spots that are usually bare in reality are bare in the game ([[Terrain Realism]])
- A pass in the editor: hand-drawn parcels following the terrain instead of a grid, and checking the forest and the chair line

## Log

- 2026-10-01: Idea.
- 2026-10-03: Built `kirkwood.save` from real elevation with a road, parking lot, beginner chair, and parcels.
- 2026-10-04: The first look showed the terrain and snow reading as uniform and plastic; planned [[Terrain Realism]].
- 2026-10-06: Kirkwood now has cliffs of bare volcanic rock, Caples and Emigrant lakes freezing through the season, Kirkwood Creek running open through a meadow, and coarse terrain when zoomed out ([[Ground Materials]], [[Creeks and Lakes]]).
- 2026-10-07: In the [[Demo]] as the main scenario.

---
title: Groups
kind: plan
status: idea
---

# Groups

Guests who come together ski together: friends and families arrive in one car, ski as a group led by one member, queue and ride together, eat together, and go home together. Only the leader does the costly thinking (planning, routes around trees, the full steering fan). The others follow the leader's line with their own bodies: their own physics, tracks, snow wear, collisions, and falls. So a group looks and plays like several skiers and costs about as much as two. Part of [[Crowd Scale]]; touches [[GOAP]], [[Skiing]], [[Transit]], [[Lifts]], [[Moments]], and "Children and families" in [[Next Steps]].

## Why

From the user, 2026-10-09: groups are a forgotten feature, and if they put more skiers on the piste, that's fantastic. The aim from [[Crowd Scale]] and [[Season Calendar]] is 10,000 visitors a day with a day in 80 seconds at the fastest speed; 1,000 now takes 63 s of sim and 102 s rendered on the Boreal Goals Test, and the cost grows about in line with skiers. Making everything faster (the rest of the step on the cores, the sim on its own thread, compact data) might give 3–4× more. That reaches about 3,000 a day, not 10,000.

The user wants to keep three things for every skier: their tracks (what the player sees), collisions (gameplay), and simulated skiing that sits on the terrain. Groups keep all three. They save on deciding, not on bodies.

## Where a skier's cost goes

Measured 2026-10-09 at the Christmas peak (about 850 on the mountain), in seconds per game hour across the cores:

| Part | Cost | Per |
|---|---|---|
| Steering (`decide`, 168 hazard samples a step) | about 1.5 | mind |
| Route searches around trees | about 1.5–2 | mind |
| Planning (goals, plan checks, boarding plans) | about 1 | mind |
| Movement, snow wear, tracks, falls | about 1.5 | body |
| Mood and conditions | about 0.3 | body |

About two-thirds is the mind. In a group, only the leader needs one.

## Leader and followers

- **The leader** is a skier as today: plans, routes around the trees, steers with the full fan, picks the line.
- **The line**: the leader drops breadcrumbs every few metres (position, speed, turn side) into a short ring.
- **Followers ski it.** Each one steers at a point on the line some way behind the leader, plus its own sideways offset (pure pursuit), with no fan, route search or planner. Each keeps its own:
  - **Physics**: speed from its own slope, snow and skill, as today, so it stays on the terrain.
  - **Tracks**: its own line, offset and in its own turn rhythm, so a group leaves parallel tracks as real groups do.
  - **Snow**: wear and packing under its own skis.
  - **Falls**: from its own balance.
- **Collisions per body, cheaply.** A follower checks only where it is: the trunk field for a tree and the skier grid for anyone near, both already built each step. In trees the group closes up into single file on the leader's line, which already missed the trunks. Hits and falls work as today. A follower swerves with a small fan (a few samples, not 168), and only when something is close.
- **Spacing**: followers keep a gap from the member ahead, so the line doesn't bunch.
- **Not a formation**: varied offsets, turn phase and gaps, catching up after a fall, and stopping to wait, so a group reads as people skiing together.

## What a group decides together

- **Terrain**: the leader plans for the weakest member: a family sticks to greens.
- **Pace**: the group waits for anyone behind at junctions and the bottom of a run, and regroups in the lift line.
- **Needs**: each member keeps their own hunger, thirst, energy and patience, and their own moments and review. The group plans for whichever need presses most, so lunch is together.
- **Lifts**: the group queues together and rides together: a family of four fills a quad, a pair shares a chair, and singles fill the gaps.
- **Splitting up**: with a big skill gap, the strong skier can ski alone and meet the others for lunch or at the car (a group of one, then a regroup). That's realistic, and a player choice: resorts with terrain for everyone keep families together.
- **Moments to discover**: "skied with friends", "waited for the kids at every turn", "lost the group", "the whole family on one chair".

## Who's in a group

Today a carload is the poll's winners from one entry, one to four to a car (mean 2.4), strangers drawn fresh each visit ([[Transit]]). Groups make that real:

- **A group is a carload**, decided when the guests are rolled into the pool: friends, couples, families. Members visit together, so a group's visits are one poll roll, not one per guest.
- **Families**: children as a guest type (Next Steps, Guests). They're beginners and need childcare or ski school ([[Childcare]], [[Ski School]]) for parents to ski alone. That's later; groups come first.
- **Solo skiers stay**: a group of one is a leader with no followers, as today.

## The gain

If a follower costs 20–30% of a full skier, a group of 2.4 costs about (1 + 1.4 × 0.25) ÷ 2.4 ≈ 0.55 of today per guest, about 1.8×. With the rest of the step on the cores and the sim on its own thread, that's maybe 5–7× in all: several thousand visitors a day at 80 s. A follower's real cost is the first thing to measure.

## Risks

- **Formation look**: identical offsets and timing read as scripted. Variety and waiting should cover it; check on screen.
- **Followers on the wrong terrain**: they ski a line picked by someone else. Planning for the weakest member covers most of it; a follower far past their comfort pitch can drop back and traverse.
- **Bunching on the line**: spacing from the member ahead, using the skier grid.
- **Saves**: groups, leaders, and breadcrumbs save and load; old saves break (fine before release).

## Steps

1. **Prototype on a testbed**: one group of three on a groomed run, leader as today, two followers on its line. Measure a follower's cost per step against a full skier's; look at the tracks, the motion, and the collisions with a few solo skiers on the run.
2. **Groups in the pool and the car**: a group is decided when guests are rolled into the pool; demand rolls a group's visit once; the car carries its group.
3. **Skiing together**: leaders and followers on every descent; trees in single file; spacing; waiting and regrouping.
4. **Lines and lifts**: queuing together, chairs filled by group.
5. **Planning together**: terrain for the weakest member, needs pooled, meals and going home together; splitting up and meeting again.
6. **Moments and readouts**: group moments; a group shown in the guest popup.
7. **Measure** on the Boreal Goals Test: a 1,000-visitor day, then as many as the fastest speed will carry.

Each step builds with `go build` and `go vet`, and is checked headless and on screen. No Go tests.

## Open questions

- Group sizes: the carload mix today (one to four, mean 2.4), or bigger groups in two cars.
- Whether followers ever choose their own line within the run (more natural, more cost).
- How far the group waits for a slow member, and when they split up.
- Whether a group's members share a review or each leave their own (each, today's plan).

## Log

- 2026-10-09: Written with the user after the [[Crowd Scale]] passes: one detailed sim for every guest, with followers sharing the leader's thinking; keep tracks, collisions, and terrain-grounded skiing per body.

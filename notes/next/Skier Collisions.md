---
title: Skier Collisions
kind: plan
status: in progress
---

# Skier Collisions

Skiers who fail to avoid each other collide: both go down, and sometimes one is hurt. Collisions come from the skiers themselves (speed, skill, how late they saw the other person, how crowded it is), not from a dice roll per run, so they cluster where the player's layout causes them: merges, narrow cat tracks, the runout above the lodge, a fast black emptying onto a green. Part of [[Skiing]], [[Ski Patrol]] and [[Moments]]. *For* the merge-zone pushback in [[Vision]] and [[Alta]]; it's the reason **Slow zones** (in [[Next Steps]]) exist.

## Why

Read on 2026-10-14:

- **Skiers dodge each other but never touch.** `swerve` (`internal/sim/ski_turns.go`) looks `swerveLookSec` (1.5 s, at least 5 m) along the line. If another skier is on it, the guest pivots to the nearest clear heading (1.5 m clearance), or slows to the 1 m/s creep (`skiCreepSpeed`) when there's no way past. Nothing happens when the dodge fails: two bodies can overlap and ski on.
- **Everyone sees perfectly.** Every skier sees every other skier within reach, in every direction, from where they stood at the start of the step (`ski_parallel.go`). Nobody has a blind spot, nobody is slow to react, and nobody misjudges.
- **Trees already show the pattern.** `treeHit` (`internal/sim/skiing.go`) knocks the guest down (`knockDown` with `FallThrown`), gives the thought "I hit a tree!", and rolls an injury chance that grows with speed (`injure`: a serious injury waits for patrol, a minor one goes home). Collisions reuse all of that.
- **Vision already promises it.** Vision's progress table lists "Merge-zone collisions: partial — skier-vs-skier hazard exists in steering; needs a fall/injury consequence and a slow-zone tool".

## The model

**Contact.** After the step's moves are applied (the serial pass in `ski_parallel.go`, so seeded runs repeat), check each skiing guest against the skier grid for anyone within a contact radius (about 0.8 m, two bodies with skis). Each pair resolves once, in a fixed order. Guests standing in lift lines, loading, unloading or walking are skipped; fallen guests count (skiing into someone lying on the slope is a real collision).

**How hard.** What matters is the closing speed along the line between them, not either skier's own speed:

- **A brush** (slow closing, under about 2 m/s): no fall. Both lose some balance, so a shaky beginner may still go down.
- **A collision**: both go down. The one who hit falls forward (`FallForward`) and the one who was hit is thrown (`FallThrown`) along the impact, so they slide apart rather than through each other. Each rolls for an injury that grows with closing speed (like `treeHit`, with its own max chance and serious share). The skier who was hit is more likely to be hurt: they didn't see it coming.

**Who's at fault.** The skier code: the uphill and overtaking skier gives way. The skier whose heading points more at the other, or who is moving faster along the line between them, is the one who hit. Fault decides the thoughts and the injury odds.

**Why the dodge fails.** If avoidance stays perfect, contact never happens. So steering gets realistic limits, scaled by skill and taste, and collisions come out of them:

- **Looking ahead, not behind.** A skier sees others within a forward cone (about ±100°) and not behind. The downhill skier doesn't dodge someone coming from above; that's the uphill skier's job, as in the skier code.
- **Reaction time.** Others are seen as they were a moment ago: about 0.6 s for a novice down to 0.2 s for an expert. With a 1.5 s lookahead that rarely matters at cruising speed, and it matters a lot to a fast skier closing on a slow one.
- **Speed beyond the turn.** This already exists: a skier going faster than their pivot rate can handle finds no clear heading and has to brake. Braking is limited by grip and snow, so on ice the creep may not come in time.
- **No gap.** In a crowd every heading is blocked. The creep at least makes contact slow, which is usually a brush.
- **Tastes.** Guests with a high Steep taste (Chargers, [[Snow Tastes]]) already ski faster, so they close on others faster and collide more without any new rule. No separate "reckless" trait: the user wants that with **Snowboarders** ([[Next Steps]]).

**Groups.** Group members dodge and collide like everyone else ([[Groups]] keeps collisions per body). A follower tucked in on the leader's line counts as part of the group and doesn't collide with that leader unless closing speed is high.

**What the guest says.** New thoughts (`ai.ThoughtKind`):

- Hit: "Someone crashed into me!" (annoyance, stronger than hitting a tree, aimed at the resort's crowding) and, when hurt, the injury thoughts as today.
- Hitter: "I ran into someone." (a letdown).
- A brush: "That was close." (a small annoyance; frequent ones on a run tell the player it's crowded before anyone gets hurt).

**What the player sees.**

- The fall record gets a cause (`world.FallRecord`: `Cause` = balance, tree, collision, ...), and the day's falls per run (trail popup, History) split by cause. This covers the "a fall's cause named" part of *Trail steepness for the player* for collisions and trees.
- Collisions are markers on the falls overlay at the spot, so a merge lights up.
- A serious collision injury calls [[Ski Patrol]] as a fall does today.
- The guest popup's event list shows "Collided with <name>" for both.

## Steps

1. Done: **Count contact as it is today.** `sim/contacts.go` counts contacts with no consequences, printed by `tools/days` each day and by headless testbeds; new testbed **Trail merge, intermediate and advanced skiers**. Findings below.
2. Done: **Consequences.** A contact closing faster than a body's `collideAt` (3 m/s for a novice up to 8 for an expert) knocks that body down; slower, from 1 m/s, it costs balance (up to 0.5). Group members don't collide, and slow contacts near a lift's base or top cost nothing. The hitter falls forward, the one hit is thrown along the impact; injury rolls grow with closing speed (1.5× for the one hit). Thoughts "someone crashed into me!", "I ran into someone", and "that was close" for the one brushed past at 2 m/s or more. Every fall records its cause (`world.FallCause`, saved). Boreal Goals Test, Dec 4, seeds 1–3: 28–39 knocked down against 511–579 falls (5.5–7%), 2–4 hurt.
3. *Not needed for the target (step 2 lands at 5.5–7%).* **Imperfect avoidance.** Forward cone and reaction time by skill. Measure collisions per 1,000 skier-runs on Boreal and the testbed before and after. The aim is for collisions to cluster at merges and on crowded runs and to be rare on an empty one. Target (the user, 2026-10-14): on a busy Boreal day, collisions are about 5–10% of falls, and a quiet run sees almost none.
4. **Readable.** Collision markers on the falls overlay, falls by cause in the trail popup and History, "Collided with" in the guest popup. Check with screenshots on Boreal at a merge.
5. **Tune by eye** in the running build: contact radius, brush threshold, injury chances, cone, reaction times.

## Step 1 findings

Boreal Goals Test, Dec 4 and 5, seeds 1–3 (about 1,000 visitors a day, 380–550 falls), 2026-10-14. Contacts a day, by where:

| | a day | closing speed |
|---|---|---|
| Within a group (party or class) | 6,300–7,000 | not measured |
| Near a lift's base or top (40 m) | 5,100–7,100, about 70% with someone standing | mostly slow |
| Out on the snow | 360–530 | under 2 m/s: 300–450; 2–5: 50–70; 5–10: 6–16; 10+: 0–3 |

- **Steering already leaves gaps.** Out on the snow, 60–90 contacts a day close faster than 2 m/s, which is 15–20% of the day's falls. That's already above the 5–10% target, before any imperfect avoidance. Skier-on-skier contacts are mostly crossing (115–150 a day), then same way (35–55), and rarely head on (under 12).
- **Standing guests get skied into** out on the snow (180–340 a day), mostly guests heading to a lift, taking skis off or idle near the bases, just outside the 40 m.
- **One hot spot holds on every seed:** Sunset Blvd. about 90 m below Lift3's top, at (870–890, 1170): 60–160 a day. Then Lift1's base (730, 430) and Lift4's base (930, 460).
- **Groups overlap constantly**: followers ride on the leader's line, about 6,600 contacts a day.
- **Merge testbed:** 20 skiers off a single chair (one every 7.5 s) over 25 minutes: 6 contacts, all at the merge, crossing, up to 6.7 m/s. The chair spaces skiers out too much to crowd the merge.

What this means for the next steps:

- **Step 2:** contacts within a group, and slow ones at the stations, don't count as collisions. Otherwise base areas would be full of falls, and a group would knock itself over all day.
- **Step 3 may not be needed** to reach the target. Tune the brush threshold in step 2 first (5 m/s instead of 2 gives about 2–4% of falls), and only then decide whether imperfect avoidance is wanted for where collisions happen.

## Decided

The user, 2026-10-14:

- Collisions are about 5–10% of falls on a busy day.
- No reckless trait now; it comes with Snowboarders.
- Only skiers collide with each other. Lift towers and stations stay as they are (skiers slow to a creep and never hit them). Trees already have hits (`treeHit`), which collisions reuse.

## Log

- 2026-10-14: Planned with the user; step 1 built and measured (findings above). The merge testbed turned up three bugs, filed in [[Bugs]].
- 2026-10-14: Step 2 built. The user: get to 5–10% of falls, ideally by better skiing, else by easing slow contacts, some areas, and skilled skiers. Skill-scaled knockdown speed, no collisions within a group, and no cost for slow contacts at stations put it at 5.5–7% on Boreal with today's steering; better avoidance (skiers anticipating others' motion) is the next lever if the user wants fewer.
- 2026-10-15: Knockdown speeds raised to 5–12 m/s closing (from 3–8) when skiing physics sped guests up ([[Skiing]]: edges no longer brake a traverse); collisions back to 6–10% of falls on Boreal.

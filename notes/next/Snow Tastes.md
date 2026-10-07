---
title: Snow Tastes
kind: plan
status: in progress
---

# Snow Tastes

Give each guest a taste in snow and terrain, and let it shape their mood, their runs, and where they choose to ski. The snow simulation already distinguishes eleven snow types, grooming, moguls, and traffic, but guests only notice trees and corduroy. With tastes, grooming becomes a trade-off: corduroy pleases cruisers and erases the powder and bumps that experts came for. The player shapes the snow; guests decide where to go ([[Vision]]). Ranked second in [[Next Steps]], after [[Satisfaction Rework]], whose effects table and mood target this builds on.

## Why

Read on 2026-10-07:

- **Guests have two yes/no tastes.** `LikesGlades` (30% of advanced guests, no one else) and `PrefersGroomed` (60% of everyone) are rolled in `newPoolGuest` (`internal/world/guest_pool.go`) and saved as `glades` and `groomed`. They only move the mood target while skiing.
- **The snow has far more detail than guests use.** Each cell has a layered snowpack whose top layer is one of eleven `SnowKind`s, plus `Grooming`, `MogulSize`, and `SkierTraffic` (`internal/world/terrain.go`). Snow type only changes how fast energy and thirst drain (`energyDrainRate`, `thirstExertionMultiplier`), and moguls, powder, ice, and slush don't touch mood.
- **Guests choose runs by skill alone.** `liftAccessible` filters lifts by trail difficulty, `Explore` rides each lift once, and `GoHome` gains weight once everything has been ridden. Variety changes how long a guest stays, not how much they enjoyed it.
- **Every guest steers around trees the same way.** That was deliberate: willingness to ski glades was always meant to be its own trait, not something derived from skill.
- **Good runs aren't rewarded.** In the current model, satisfaction only rises by drifting toward a terrain target capped at 0.80, so the main thing guests come for barely registers.

## Decisions

Made with the user on 2026-10-07:

- **Tastes are continuous affinities**, each from −1 (hates it) to +1 (loves it), rolled per guest.
- **The player sees a label.** A guest's tastes show as the closest archetype's name (Cruiser, Powder Hound, and so on) wherever that helps: the follow panel, guest lists, and the thoughts chart. The sim never reads the label.
- **Taste and ability are separate.** Taste is what a guest wants; skill and `ComfortSlope` are what they can handle. A beginner can want steeps and still be scared on them. Skill tilts the roll but doesn't decide it.
- **Snow feeds the reworked layers**: conditions underfoot move the mood target, a finished run is an event, and both are rows in the [[Satisfaction Rework]] effects table.

## The model

**Tastes.** `ai.Tastes` on `GuestTraits`, replacing `LikesGlades` and `PrefersGroomed`:

| Affinity | Feature it reads (0..1 per cell or run) |
|---|---|
| Groomed | `Grooming` |
| Powder | Top layer is powder, scaled by fresh depth |
| Moguls | `MogulSize` |
| Trees | Tree cover (later, nearby trunks from [[Stored Trees]]) |
| Steep | Slope relative to the trail's difficulty |
| Ice | Boilerplate, crust, or frozen granular on top (most guests are negative) |
| Crowds | Guests on the same trail per 100 m (most guests are negative) |

**Rolling.** `newPoolGuest` picks an archetype, weighted by skill, and draws each affinity around that archetype's centre with spread, so guests blend between types. The starting archetypes, with their centres to tune by eye:

| Archetype | Leans toward | Leans away from | Most common among |
|---|---|---|---|
| Cruiser | Groomed | Moguls, powder, steep | Beginners, intermediates |
| Powder Hound | Powder, trees | Groomed, crowds | Advanced |
| Bump Skier | Moguls, steep | Groomed | Intermediates, advanced |
| Glade Rat | Trees, powder | Crowds | Advanced |
| Charger | Steep, groomed | Moguls | Advanced |

`ai.TasteLabel(t)` returns the nearest archetype by distance.

**Snow underfoot (condition).** While skiing, the mood target in `tickMood` adds Σ affinity × feature × weight over the cell under the guest. On top of that, a fear term applies when the slope is beyond `ComfortSlope`, whatever the guest's taste. Strong matches and mismatches are condition thoughts: "this powder is unreal", "loving these bumps", "these bumps are killing me", "it's sheet ice up here", "too crowded on this run", "loving these glades", "too many trees!". The existing glade and corduroy thoughts become rows of this kind. Energy drain also reads tastes: a feature a guest dislikes tires them faster.

**The run (event).** `Guest.RunGroomingSum` and `RunGroomingSamples` become a `Guest.Run` summary: the average taste match, vertical, falls, and fresh-powder cells skied. When the run ends, it produces one event: "what a run!" (+, scaled by match and vertical), "first tracks!" (+, fresh powder for a powder lover), or "that run was miserable" (−). A fall or injury on the run takes the place of the good event. Runs become the main source of good moments.

**Choosing.** Each trail keeps `Trail.Conditions`, an average of the features above along the trail, refreshed on a slow cadence (about once a game hour, and after grooming or a storm). Guests score an option as taste · conditions, scaled down by how many times they've already lapped it. `Explore` and lapping pick the best-scoring reachable lift and trail instead of any lift not yet ridden. The trees affinity also sets how hard the steering avoids trees, so glade lovers actually go into the woods.

**Variety and availability.** Each lap of the same trail is worth less, which shrinks both its run event and its score. A guest whose best option has fallen below a floor gets "skied this one to death" or "nothing here for me" (the milder, taste-based version of `ThoughtNothingForMe`). That pulls the mood target down and raises `GoHome`'s weight. It replaces "ridden every lift", so a resort with three runs gives short, flat days.

## Steps

1. Done: **Tastes and labels.** `ai.Tastes` is seven affinities indexed by `TasteKind`, with the archetype table `ai.Archetypes` (centres and shares by tier) and `ai.TasteLabel`. `world.RollTastes` rolls them in `newPoolGuest` (spread 0.25). They're saved as `tastes`, replacing `glades` and `groomed`; older saves roll tastes from each guest's ID, as legacy skills already are. The follow panel shows skill · archetype, plus a "likes:" row with the seven affinities. Until step 2, the old reactions read `Tastes.LikesGlades` (trees ≥ 0.4) and `Tastes.PrefersGroomed` (groomed ≥ 0.3); testbeds get the Cruiser centre for beginners and intermediates and neutral tastes for advanced guests, as the old defaults were.

   Checked on Boreal's pool of 10,000 (bundled and the user's save, which roll identically):
   - Beginners: 80% Cruiser, 10% Charger, 5% Bump Skier, 3% Glade Rat, 2% Powder Hound.
   - Intermediates: 52% Cruiser, 19% Bump Skier, 11% Charger, 10% Powder Hound, 9% Glade Rat.
   - Advanced: 28% Powder Hound, 22% Bump Skier, 22% Glade Rat, 15% Cruiser, 13% Charger.

   Likes glades: 4% / 15% / 35% by tier (was 0 / 0 / 30%). Prefers groomed: 86% / 59% / 26% (was 60% everywhere), which shifts the grooming pull until step 2 replaces it.
2. Done: **Snow underfoot.** `sim/underfoot.go`. Features per cell, each 0 to 1:
   - groomed: the cell's grooming
   - powder: fresh ungroomed powder, about 30 cm (0.03 m of water) for 1
   - moguls: mogul size
   - trees: tree cover
   - steep: 10° for 0 up to 35° for 1
   - ice: boilerplate or frozen granular 1, crust 0.25 (mostly corduroy firming overnight on a cold clear day, not sheet ice)
   - crowds: moving skiers within about 7 m, 3 for 1

   Each guest keeps a running average of each taste × feature over about 6 sim seconds of skiing (`Guest.Underfoot`, reset at each run's start). Reading single cells made the thoughts flicker: 13 "sheet ice" a visit before, 0.06 after. The taste term is the sum × 0.15, held to ±0.25, plus a fear term of up to −0.15 at 10° past `ComfortSlope`; it replaces the fixed grooming pull and the glade conditions' pulls (`SkiTerrainPull`). Condition thoughts start at ±0.4 and end below 0.2 or off skis, and report the term rather than carrying an effect: "this powder is unreal" / "this deep snow is wearing me out", "loving these bumps" / "these bumps are killing me", "loving these glades" / "too many trees!", "it's sheet ice up here", and "this is way too steep for me" (fear). Energy drains faster by 1 + 0.5 × the guest's dislike underfoot. `Tastes.LikesGlades` is gone; `PrefersGroomed` stays for steering and great-run reasons.

   Checked on the user's Boreal save, 2 days, groomed every night versus never. Beginners (mostly Cruisers) end at 0.55–0.58 and intermediates and advanced at 0.35–0.43 either way. Ungroomed brings bump complaints (0.6 → 1.4 a visit) and sheet ice (0.06 → 7.6, about once a run: skied snow freezes to boilerplate on cold clear days). Final satisfaction barely moves with grooming, because the pull only lasts while skiing and mood drifts back to the baseline on the lift and the walk out. Step 3 (great runs by taste) is what makes snow quality reach the baseline and the rating.
3. Done: **Run events.** `Run.Taste` is the sum of taste × feature per tick, times seconds, from `tickUnderfoot`. `Run.Fresh` is seconds on fresh untracked powder: a powder feature of 0.5 or more on a cell with `SkierTraffic` under 0.5. Snowfall now buries traffic the way it buries tracks (2 cm of water covers it fully). In `judgeRun`, the run's average taste match, clamped to −1..1, scales a great run's bonus by 1 + the match. At −0.3 or worse the run is "that run was miserable" (−0.05) and can't be great. A powder lover (powder taste 0.4 or more) on fresh powder for a third of the run gets "first tracks!" (+0.04).

   Checked on the user's Boreal save, one day with grooming off, after a 25 cm storm and without. After the storm, Cruisers on ungroomed powder were miserable on 2.55 runs a visit (0.80 without) and ended at 0.32 (0.53 without), and "this powder is unreal" came 0.69 a visit. First tracks happened about once in the day: only 2–3 Powder Hounds a day, and 77 guests track one trail within the first runs. The plan's check (Powder Hounds score higher after a storm) can't show on Boreal: almost all of them are intermediate or advanced, so on its one green they never have a great run and carry "too easy" all day. It needs a blue or black run, and the calibration that's tabled.
4. Done: **Choosing by taste.** Changed from the plan: guests never followed painted trails (every descent is free steering toward the next lift), so picking a trail by taste became steering by taste, the agentic version the vision asks for. Snow features come from one function, `cellFeatures` (`sim/underfoot.go`), used by snow underfoot, steering, and trail conditions, so the [[Moguls]] map reaches all three through `Cell.MogulSize`.
   - **Lifts.** `Trail.Conditions` is the average of each trail's cell features, refreshed every clock hour and at load. `World.LiftConditions` averages the trails off a lift's top. `RideLift.Cost` adds up to 300 s (`tasteMissSec`) for terrain that doesn't suit the guest: (1 − match) / 2, using the guest's tastes from the planner's snapshot. `Explore` stays as the novelty drive: of the lifts not yet ridden, guests take the one whose terrain suits them.
   - **Steering.** Each candidate line scores 2.5 × the sum of taste × feature along it, replacing the fixed grooming bonus (×4 for corduroy lovers). The trees taste scales how hard a guest avoids tree stands, 1 − 0.8 × trees taste (`standCoverScale`). Everyone still avoids individual trunks, and the groom-edge penalty for corduroy lovers stays.

   Checked on the user's Boreal save after a storm, with the cat grooming the trail overnight, as shares of skiing time between 9:00 and 16:00. Before, every archetype skied the same line: about 73% corduroy, 2% powder, no trees. After:
   - Cruisers: 82% corduroy, 2% powder.
   - Chargers: 84% corduroy.
   - Bump Skiers: 43% corduroy, 44% powder.
   - Glade Rats: 25% corduroy, 66% powder, 3% trees.
   - Powder Hounds: 9% corduroy, 75% powder, 5% trees.

   Boreal's lift costs a Cruiser 206 s and a Powder Hound 346 s (a mostly groomed trail); after the storm, 228 s and 311 s. With only one lift, the split between lifts can't show in play yet.
5. **Variety, availability, and crowds.** Use the per-trail run counts from [[Mood Baseline]], the boredom and "nothing here for me" conditions, `GoHome` weighted by the best remaining option, and the crowds term. Check: session length and satisfaction with one lift versus three.
6. **Docs.** Update [[Satisfaction]], [[Skiing]], [[Grooming]], [[GOAP]], [[Guest Types]], and [[Guests Spec]].

Each step builds with `go build` and `go vet` and is judged in a headless Boreal run. No Go tests.

## Folds in from Next Steps

- **What each skill wants**: the terrain half. Rentals, food, and rest stay with services.
- **Mogul lovers**: the Bump Skier archetype. The expert who entertains the lift above stays an easter egg.
- **Guest goals beyond lapping**: powder hunting. Shortest line, après-ski, and staying near the lodge stay where they are.
- **Crowding**: the run half. Crowded lodges stay with services.
- **Guests react to trunks**: the trees feature reads nearby trunks once [[Stored Trees]] supports it.
- **Rating per skill level**: unchanged, but per-archetype satisfaction from steps 2–5 is the data it will want.

## Open questions

- Whether guests know trail conditions before skiing them (a resort-wide snow report) or learn them run by run. Start with the snow report.
- Taste mixes per road entry, so a scenario's locals lean their own way: bump skiers at [[Mad River Glen]], powder hounds at [[Alta]] and [[Asahidake]], cruisers near a city. This needs only a mix on each entry's pool.
- How [[Guest Types]] snowboarders differ: likely more powder and less moguls, once they ride as snowboarders.
- A terrain park affinity once there is a terrain park.
- Whether tastes drift with visits, as regulars get better.

## Log

- 2026-10-07: Planned with the user: continuous affinities with archetype labels for the player. Ranked second, after [[Satisfaction Rework]].
- 2026-10-07: The run summary and judging shipped with [[Satisfaction Rework]]; step 3 now extends them with tastes.
- 2026-10-07: Step 1: tastes, archetypes, labels, saving, and the follow panel.
- 2026-10-07: Step 2: snow underfoot against tastes, smoothed over a few seconds; its pull is momentary until step 3.
- 2026-10-07: Satisfaction became a ledger; snow underfoot's pull is gone, and step 3 carries snow into the score through run verdicts.
- 2026-10-07: Step 3: run verdicts by taste (great runs scaled by the match, miserable runs, first tracks); snowfall buries skier traffic.
- 2026-10-07: [[Moguls]] planned; the moguls feature stays `Cell.MogulSize`, which the mogul map will feed as its cell average.
- 2026-10-07: Step 4: lift choice by trail conditions and steering by taste (guests never followed trails); glade lovers go into the woods.

---
title: Snowmaking on Runs
kind: plan
status: partial
---

# Snowmaking on Runs

Snowmaking as real resorts do it: pipe and hydrants along a run, which then makes snow over the whole run on cold nights, rather than guns placed one at a time. Replaces the placed snow guns in [[Snowmaking]]; touches [[Snow]], [[Trails]], [[Grooming]], [[Lifts]], [[Finance]], and [[Night Sessions]] ([[Night Skiing]]), which leans on it to stay open late in a thin season.

## What's there today

Surveyed 2026-10-14. The whole mechanic is `tickSnowGuns` (`sim/snowgun.go`), and it barely matters:

- **Tiny reach.** A gun covers a 15 m disc (725 m²). A 1 km run 25 m wide needs about 35 guns: $1.4M to build and $42k a day to run, against $900–2,400 a day for a lift.
- **Little snow.** 3 mm of water an hour at any temperature at or under −2 °C, laid as dense base (0.75), so a cold twelve-hour night adds about 5 cm (the card says 9). The same rate at −3 °C as at −20 °C; the base's temperature is used however high the gun is.
- **Costs don't follow use.** $1,200 a day while switched on, too warm or not.
- **It undoes the cats.** Every cold tick resets a groomed or powder top layer to base, leaving the corduroy value behind.
- **Nothing depends on it.** A lift holds only when its base has no snow at all, so a single gun-hour beats a hold; thin runs ski like deep ones.
- **No feedback.** No overlay, events, or totals.

## Decisions

From the user, 2026-10-14:

- **On runs.** The player buys snowmaking for a run (pipe and hydrants, by the metre), and the run makes snow over its whole area when it's cold enough. The placed snow gun goes away (pre-release: old saves lose theirs).
- **Paid by the hour.** Snowmaking costs only while it's making snow, by how much snow it makes, and the player can switch it off for a run to save money. No pump or water limit.
- **Real depth matters.** Lifts hold under a real base depth at their base, and thin runs show rock: guests avoid them and complain. Natural-snow resorts open later; snowmaking earns its keep.

## How it works

- **Installing.** A run's popup offers snowmaking at a price per metre of run (about $250 a metre: $250k for a kilometre). Installed, it has an on/off switch and a target depth (default 60 cm); `Trail.Snowmaking`, `Trail.SnowmakingOn`, `Trail.SnowTarget`, saved.
- **Making snow.** When the air at a cell is cold enough (the base temperature corrected for altitude, 6.5 °C a kilometre), each snowmaking cell below its target gets machine snow at a rate rising from nothing at −2 °C to full at −10 °C: full is 8 mm of water an hour, so a cold twelve-hour night lays about 20 cm. Cells at their target stop, so a deep run costs nothing.
- **When.** Only while the run's lifts are stopped (nights and closed days; not while it's lit for [[Night Sessions]]), stopping at OpenHour − 2 so the cats can groom over it.
- **Cost.** By the water made: $2 a cubic metre (a kilometre run 25 m wide on a cold twelve-hour night: about 2,400 m³ of water, $4,800), as the "Snowmaking" cost line.
- **Machine snow.** Its own layer kind (`KindMachine`, 0.45 when laid, settling like any snow): firm and fast. It takes in the surface layer it's laid on rather than folding it into the base, which would pack it to base density and leave the run shallower than before. It clears the corduroy; skiers and cats pack it to packed powder; cold, clear and windy days leave it, a warm one turns it to corn.
- **Guns you can see.** Fan guns drawn along a snowmaking run's edge every 40 m (reusing the snow gun model), blowing a plume of particles while they make snow.
- **Depth that matters.** A lift holds when the base of its line has under 30 cm of snow (`LiftHoldDepth`). A run cell under 20 cm shows rock: route costs avoid it, and skiing it can bring "hit rocks on <run>" (a letdown) and a fall now and then.
- **Feedback.** A Snowmaking map view (runs by depth against target, blowing now); the run popup shows depth, target, snow made and cost last night; the morning feed says what was made ("Snowmaking laid 18 cm on Kiss A Bear overnight, $2,700").

## Steps

In order; each builds with `go build` and `go vet`, and is checked headless on the Boreal Goals Test and on screen.

1. Done: **Snowmaking on runs**: the run's install, switch and target; snow by temperature and altitude to the target, while its lifts are stopped; cost by water made; the placed gun removed. Machine snow as its own kind.
2. Done: **Depth that matters**: lift holds under 30 cm; rock on thin runs (routes, letdown, falls).
3. Done: **Guns and feedback**: guns along the runs with plumes, the Snowmaking map view, the popup readouts and the morning feed line.
4. **Balance**: an early-December Boreal opening on snowmaking alone; the cost per run per night against a lift's day; [[Boreal Tutorial]]'s money.

## Open questions

- Whether Auto snow's season replay ([[Terrain Layers]]) should include snowmaking, so a scenario can open with a man-made base.
- Whether the demo gets snowmaking back (it was left out with the gun).

## Log

- 2026-10-14: Surveyed and planned with the user, ranked first, ahead of [[Night Sessions]].
- 2026-10-14: Step 1 built. On the Boreal Goals Test (Dec 16, every run on, target 2 m, closed): the first cold night (−5 to −12 °C) made snow on all 11 runs, about 275,000 m², for $42,755; the next two milder nights $11,027 and $6,558. Runs gained 13–26 cm a night over the same days without snowmaking, but both lose about half their depth to the compaction bug (0.89 m to 0.41 m in a day without snowmaking), which step 2 waits on.
- 2026-10-14: The compaction bug fixed ([[Snow]]): layers keep their own density, so step 2's depth thresholds hold still.
- 2026-10-14: Step 2 built. Lifts hold under 30 cm at the base, back from 35 cm; rock under 20 cm: route cost, "hit rocks, the snow's too thin" (an annoyance), and falls (a fifth of hits, less for good skiers). The shipped scenarios: the Goals Test's lift bases are at 1.1–1.2 m and Boreal's ground about a metre everywhere, but Kirkwood's lowest tenth is mostly under 30 cm on its Dec 10 start, so lifts based there wait for storms or snowmaking. The Goals Test with every run cut to 10 cm (lift bases left deep): 334 rock complaints, falls 624 → 3,493 and 2.90★ → 2.56★ at a 30% fall share, since cut to 20%.
- 2026-10-14: Step 3 built: guns every 40 m on a run's right-hand edge with see-through plumes while making snow, the Snowmaking map view, the popup's last night, and the morning feed line.
- 2026-10-14: Good for now (the user); balance is listed under Economy in [[Next Steps]].

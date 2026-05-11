# Skier AI — pipeline overview

Continuous steering controller. The pipeline runs once per agent per tick
from `tickSkier` in `internal/sim/skiing.go`. There is **no technique enum**
and **no waypoint planner** — S-turns, brake wedging, and tree avoidance all
emerge from a single steering function reading a small typed perception
bundle.

Persistent per-agent state (`Traits`, `Plan`, `Balance`, `TurnSide`,
`TurnDwell`, `LastTactical`, `Energy`, `Sense`) lives on `world.Agent`.
Per-tick types (`Perception`, `Decision`) are sim-internal and never stored.

```mermaid
flowchart TB
  Start([tickSkier · per agent · per tick])

  Start --> Arrival{dist &lt; ArrivalThreshold?}
  Arrival -->|yes| Done([snap to target · return arrived])
  Arrival -->|no| PlanRefresh

  PlanRefresh["<b>Plan refresh</b> — on goal change only<br/>strategic layer is intentionally thin;<br/>per-tick controller never re-reads goal"] --> L1

  subgraph L1["L1 · Perception — every tick"]
    direction LR
    Terrain["<b>Terrain</b><br/>NormalAt (snow surface) · SlopeAngle<br/>FallDir · FallScale (smoothstepped [0,1])"]
    Axis["<b>Axis target</b><br/>vector to a.Plan.Target<br/>→ AxisDir · AxisDist · InArrival (&lt;6 m)"]
    Under["<b>Underfoot</b><br/>TreeDensityAt(pos) → AtCellDensity<br/>InTrees flag (display only)"]
  end

  L1 --> L2

  subgraph L2["L2 · Controller (decide) — Perception → Decision"]
    direction TB
    Axis2["<b>composeAxis</b><br/>blend(AxisDir, FallDir·FallScale)<br/>→ axisHeading"]
    Tact["<b>sampleTactical</b><br/>7 candidate offsets · ±60° · 8 segments<br/>horizon = speed·3.5 s clamped [22, 70] m<br/>corridor ±10 m perpendicular — worst-of-three density<br/>score = progress·cos(off) − 4·Σdensity − 8·boundaryHits<br/>+ 0.5·Σgrooming  <i>(centre-only)</i><br/>+ 0.4·sign(prev)·sign(off)  <i>(gated on maxDensity&gt;0.5)</i><br/>+ tiny RNG jitter to break symmetric ties"]
    Speed["<b>targetSpeed</b><br/>ComfortSpeed·(0.7 + 0.6·Aggression)<br/>× 0.5 if InArrival<br/>× (1 − 0.4·clamp(worstProbe/0.4))<br/>floor at skiWalkSpeed (2 m/s)"]
    Brake["<b>brakeAngle</b><br/>overspeed = (Speed − target)/target<br/>brakeAngle = clamp(overspeed·1.5, 0, 40°)"]
    Side["<b>TurnSide commit</b> — persistent, dwell-gated<br/>dropped to 0 when obstacleSeen OR brakeAngle &lt; 8°<br/>flips to opposite when |deviation| &gt; 0.85·brakeAngle<br/>AND TurnDwell ≥ 1.2 s<br/>initial side from heading deviation, else coin-flip"]
    Out["<b>desiredHeading</b> = axisHeading + tactical + side·brakeAngle<br/><b>scrub</b> = 4·(overspeed − 0.6) clamped to 6 m/s²<br/>active only above 60% overspeed"]
    Axis2 --> Tact --> Speed --> Brake --> Side --> Out
  end

  L2 --> L3

  subgraph L3["L3 · Apply — physics integration"]
    direction TB
    Head["<b>heading</b><br/>rotateToward(desired) capped at headingRateMax (40°/s)<br/>— matches realistic edge-transfer rate"]
    Fric["<b>effectiveFriction</b> — snow-modulated (muBase, muEdge)<br/>Grooming · Packed · Powder gate · MogulSize · Ice<br/>each shifts the corduroy baseline (see SNOW.md)"]
    Accel["<b>acceleration</b><br/>a = g·sinθ·cos(off) <br/>− μ_base·g·cosθ<br/>− μ_edge·g·cosθ·|sin(off)|<br/>− k_drag·v²<br/>− dec.Scrub"]
    Pos["<b>position</b><br/>pos.xz += (sin h, cos h)·speed·dt<br/>pos.y = surface elevation<br/>floor speed ≥ skiWalkSpeed (2 m/s)"]
    Head --> Fric --> Accel --> Pos
  end

  L3 --> BalChk{Balance += stressDelta·dt<br/>≤ 0?}
  BalChk -->|yes| Fallen["<b>Fallen</b> · 4 s<br/>resume at Balance 0.7 · TurnSide 0"]
  BalChk -->|no| Snap["<b>recordFrame → Sense</b><br/>follow HUD · perception-cone shader · CSV recorder"]

  classDef layer fill:#0f172a,stroke:#475569,color:#e2e8f0
  classDef phys fill:#7c2d12,stroke:#fb923c,color:#fed7aa
  classDef gate fill:#1f2937,stroke:#9ca3af,color:#f3f4f6
  class L1,L2 layer
  class L3 phys
  class Arrival,BalChk gate
```

## Notes on the architecture

- **Plan A — no technique enum.** Straight, carved, and brake-heavy outputs
  all come from one steering function. The brake angle (`TurnSide ×
  brakeAngle`) is what produces emergent S-turns: while overspeed,
  brakeAngle > 0 → desired heading is off the fall line → edge friction
  scrubs speed → speed drops → brakeAngle shrinks → if heading has reached
  the arc edge on the committed side, flip TurnSide and carve back.
- **No path planner.** `a.Plan` only tracks the current goal target. There
  are no waypoints, no routes. The controller seeks the goal directly and
  lets `sampleTactical` deal with obstacles in front of it.
- **Single forward sampler.** `sampleTactical` scores 7 candidate arcs at
  ±60° around `axisHeading`. Each arc is 8 segments deep; every segment
  reads tree density at the centre **and** at ±10 m perpendicular, taking
  the worst — so a path that grazes a tree edge scores as poorly as one
  through the trunk. Boundaries get an 8× penalty, tree density a 4×
  penalty, on-axis progress a +0.3 bonus.
- **Side-commit on obstacles only.** A small bias (+0.4 × sign(prev) ×
  sign(off)) keeps the skier on the same side they chose last tick — but
  only when the fan actually sees an obstacle (`maxDensity > 0.5`). Without
  that gate, `prevTactical` would self-perpetuate and slowly drift the
  skier off-axis even on a clear slope.
- **S-turn suppression while avoiding.** When `obstacleSeen`, TurnSide is
  forced to 0 — the tactical offset already takes the heading off the fall
  line, so cross-fall friction still scrubs speed, and the S-turn
  oscillation would otherwise fight the lateral commitment by swinging
  heading back through axis every cycle. Real skiers don't S-turn through
  trees.
- **Turn dwell minimum.** A committed turn side can't flip again until 1.2 s
  has passed (`turnDwellMin`). Combined with the 40°/s heading rate cap
  this puts each carve at ~1.2 s minimum and a full S-cycle at ~2.4 s —
  cruising rhythm, not slalom.
- **Snow-modulated friction.** `effectiveFriction` reads `SnowAt(pos)` and
  shifts the (muBase, muEdge) pair per Grooming / Packed / Powder /
  MogulSize / Ice. See `SNOW.md` for the multiplier table.
- **Grooming preference in steering.** `sampleTactical` integrates per-
  segment `Grooming` along each candidate arc (centre-only — the edge of
  a groomed strip is still groomed) and adds `0.5 · Σgrooming` to the
  score. On clear slopes this pulls the line onto corduroy; when trees
  are present the 4× density penalty dominates and the grooming term
  just biases tie-breaks. Uniform across skiers — `GroomingPreference`
  trait is deferred.
- **Balance + fall** runs every tick orthogonally to L1–L3. Drains from
  speed/slope overshoot, hard scrub under load, and underfoot tree density
  above 0.3. Recovers at +0.15/s baseline, clamped to [-1, 0.4]/s.
- **Energy** is a session-level fatigue budget. Drains at a flat rate per
  sim-second only while `tickSkier` is on the dispatch path (lift rides
  and walks don't drain). Fresh = 1.0; budget covers `energyBudgetSec`
  (~800 s, calibrated for ~20 descents). Once below `energyLowThreshold`
  (~0.05), decision boundaries outside the skier pipeline reroute the
  agent home: `pickTopTarget` picks a lodge at lift unload, and
  `onArrive(targetLift)` paths the skier to a lodge instead of queueing.
  The skier pipeline itself never reads `Energy`.
- **Lift selection.** `pickTopTarget` chooses the next destination at
  every lift unload. Above the energy threshold it picks any lift in the
  resort uniformly at random; below it picks a lodge.

## Future extension points

| Trait | Effect | Status |
|---|---|---|
| `GroomingPreference` | Per-skier weight on the grooming bonus in `sampleTactical` | Deferred (uniform for now) |
| `GladeTolerance` | Shifts the corridor/density penalty per-skier; advanced glade skiers tolerate density up to ~0.8 before going aversive | Deferred (constant for now) |
| `PreferredSide` | Replaces the symmetric-tie coin-flip in `pickInitialSide` with a per-skier preference | Deferred (random for now) |

All three are deliberately decoupled from `SkillLevel` — they're
personality dimensions, not skill markers.

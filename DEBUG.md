# Debug Utilities

All debug features are only active during normal interactive play (not headless/testbed mode).
Most require **following a guest** first — click a skier to lock the camera to them.

---

## In-game hotkeys

| Key | What it does |
|-----|-------------|
| **F3** | Toggle steering debug overlay — draws the fall line (cyan), desired heading (yellow), and three probe rays (red/brown) in world space above the followed skier. |
| **F4** | Toggle planner debug panel — shows GOAP goal weights, the full action plan with step costs, and the extracted world snapshot (anchor IDs, Energy, Fun). |
| **F5** | Toggle terrain inspector — shows per-cell snow/grooming/tree state under the mouse cursor, plus FPS and per-frame update/render wall time. |
| **L** | Toggle CSV log of the followed skier. Starts recording to `debug/skier-{ID}-{timestamp}.csv`; stops on a second press, if you unfollow, or if the skier departs. |
| **F12** | Capture a screenshot to `debug/screens/{timestamp}.png`. |

---

## F4 — Planner panel

Shows three sections for the currently followed guest:

**Goal weights** — every GOAP goal ranked by weight; the winner (highest-weight unsatisfied goal) is marked `>`. Goals already satisfied are labelled `(satisfied)`.

**Snapshot** — the `Extract()` output used at the last replan: anchor IDs (`AtLiftBase`, `AtLiftTop`, `AtTrailEnd`, `AtLodge`, `AtParking`), Energy, Fun, Skill. An all-zero anchor block means the guest is in transit (walking a pathfinder path or mid-ski with no settled anchor).

**Plan** — the current action sequence, up to 12 steps shown (then `… +N more`). Each line shows the action name with step cost. The head step (currently executing) is step 0.

---

## L — CSV skier log

Records one row per skiing tick to a CSV in `debug/`. Useful for analysing the L1 controller, fall-line tracking, and speed regulation.

**Columns:** `sim_t`, `agent_id`, `activity`, `pos_x/y/z`, `heading_rad`, `tgt_x/y/z`, `dist`, `speed`, `fall_x/z`, `axis_head`, `desired_head`, `target_speed`, `brake_rad`, `turn_side`, `mode`, `balance`, `probe_c/r/l`, `slope_cos`, `in_arrival_radius`

The log only captures frames in which the guest is actively skiing (not walking, queuing, or riding). Open it in any CSV viewer or load into Python for trajectory analysis.

---

## F3 — Steering overlay

Three line segments drawn from just above the followed skier's head:

- **Cyan** — fall line: gravity-projected downslope direction.
- **Yellow** — desired heading: the L1 controller's current target direction after hazard avoidance and fall-line blending.
- **Red/brown** — probe rays: the three lookahead directions sampled for hazard density (trees, other skiers). Probe length scales with speed (5–20 m).

Press F3 again to clear.

---

## F5 — Terrain inspector

Shows the cell under the mouse cursor. Key fields:

- **Ground / Surface elevation** — raw rock bed vs. snow top (metres)
- **Snow accumulation** (SWE, metres) and derived **visible depth**
- **Packed density**, **Grooming**, **MogulSize**, **Ice**, **TreeDensity**
- **Passable** — false means pathfinder treats the cell as a wall

Also shows **FPS** (smoothed) and per-frame **update / render** wall time in ms.

---

## CLI flags

Run `./mountain-mogul -help` for the full list. Commonly useful ones:

| Flag | Purpose |
|------|---------|
| `-trace` | Start a pprof endpoint on `localhost:6060` and enable slow-frame stderr logging. |
| `-cpuprofile FILE` | Write a CPU profile to FILE on exit (play normally, then quit). Inspect with `go tool pprof -http=:8080 FILE`. |
| `-memprofile FILE` | Write a heap profile on exit. |
| `-profile` | Run a 50× headless simulation benchmark (no window). Writes `cpu.prof` and `mem.prof`. |
| `-testbed NAME` | Load a registered testbed by name instead of the default scenario. |
| `-screenshot FILE` | Render and capture a PNG without interaction. |
| `-overlay-mode N` | Bitmask of terrain overlays to enable in screenshot mode (contour=1, slope=2, snow=4, grooming=8, packed=16, ice=32, moguls=64). |

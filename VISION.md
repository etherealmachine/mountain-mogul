# Vision — The Road to the Gondola

This document describes the happy path for the next iteration of Mountain
Mogul: how a player goes from an empty mountain to a resort with a few
high-speed chairs and one long gondola. It is written from the resort's
point of view — what gets built, what pushes back, what the player has to
get right, and when. It is deliberately a story, not a spec; the coding
steps get derived from it afterwards.

Numbers in here are indicative. They exist to make the arc concrete and to
show the *shape* of the economy, not to be typed into constants.

---

## 1. Premise

The player is handed a real mountain (imported heightmap) split into
parcels. The **base parcel** is owned: a valley floor with road access,
gentle lower slopes, mixed forest. Above it sit two or three
**purchasable parcels** — a mid-mountain bowl, a treed ridge, and a high
alpine face across a gap that no chairlift can span. Everything else is
off-limits.

Starting resources are enough for one modest lift pod and nothing else:
cash on hand plus a credit line at a rate that makes borrowing a real
decision, not free money.

There is no fixed season. The calendar runs continuously through the year
and the resort is open whenever it has snow under a lift — which is
decided by weather, altitude, and whatever the player does about it.
The game starts about a week before nightly lows first drop below
freezing. The player pauses, lays out the base area, and waits: for
natural snow, or — once they own snow guns — for the first cold night
they can make their own. At the other end, the resort stays open as long
as the base holds; a deep man-made base laid down in January can carry
the lower mountain well past the point where the neighbours have closed.

Building is instantaneous and costs only money. Construction time is not
a mechanic. What fills the gap between seasons is a **skip** control:
fast-forward to a date, or to a condition ("first freezing night",
"first natural snowfall", "base depth ≥ 30 cm on the green"). Closing
the resort in spring, skipping to October, and reopening should take a
few clicks.

A "season" is therefore a derived thing — the span between the first and
last day a lift ran — and the report card, pass expiry, and interest
settlement hang off that derived boundary, not off a calendar date.

One in-game day is one real minute at default speed, so a season is
roughly three hours of play. The arc below spans five seasons.

The end state — the definition of "done" for this iteration — is:

- Two base pods connected by a gondola of 2 km or more.
- Three or more high-speed lifts.
- Green, blue, and black terrain, all groomed and patrolled.
- 500+ guests on a weekend day, resort rating sustained at or above 0.8.
- A base area that guests walk through: parking → rental → tickets →
  lift, with the queues at each door visible and managed.

---

## 2. The Arc

### Season 1 — Opening Day

*"Get a lift turning before Christmas."*

**What gets built.** The game opens paused on a bare mountain in
October. The player lays out the base area before the first snow. They
place a **parking lot** at the road, a **ticket office**, and
paints a **pedestrian path** from the lot past the ticket window to the
bottom of the first lift. That first lift is a **fixed-grip quad** — the
double is cheaper but the quad is the right call if the player can
stretch to it. Its top station has to be sited so that a **green trail**
and a **blue trail** can both be painted back to the base from the same
unload point; a lift whose top only serves one difficulty will only ever
attract one kind of guest. An **equipment shed** with a single snowcat
rounds out the opening kit. There is no lodge yet; guests eat in their
cars. Then the player skips forward to the first storm and watches the
lift start turning.

**What pushes back.**

- *Waiting for snow.* Without snowmaking, opening day is whenever the
  weather says it is. A dry November means the resort sits idle while
  the first interest payment comes due. The skip control makes this
  painless mechanically; the cost is entirely financial, and it is the
  first argument for snow guns.
- *Cash flow.* The opening build consumes almost everything. The first
  weeks run at a loss (attendants, cat fuel) against tiny weekday crowds.
  The credit line is the bridge; the interest is the first recurring cost
  the player learns to hate.
- *The Christmas week crush.* Demand roughly doubles on weekends and
  triples over the holiday. The single quad's queue hits the cap, guests
  turn around in the parking lot, and the rating takes its first dip.
  This is the moment the player discovers the event feed: "Lift 1 queue
  over 20 — guests leaving."
- *The first storm.* A heavy-snow day buries the corduroy. If the cat
  wasn't running that night, the beginners spend the morning falling in
  powder and the exit-thoughts chart fills with "ouch." If it was, the
  day after the storm is the best day of the season — powder for the
  intermediates, fresh groom for everyone else, and the biggest crowd yet.
- *The first injury with nobody to help.* Someone falls hard on the blue.
  Without a patrol hut they wait, give up, and crawl home. The thought is
  brutal and the rating hit is bigger than a long queue.
- *Thirst and hunger.* Guests leave earlier than they want to because
  there is nowhere to eat. Session length caps the revenue per ticket.

**Key decisions.**

1. **Where the base triangle sits.** Parking, ticket office, and lift
   base need to be close — every metre of boot-walking drains patience.
   But the lift base needs a run-out that green skiers can reach, and the
   lot needs road access. Getting this triangle right is the single most
   important decision of the game; everything later hangs off it.
2. **Fixed quad versus double.** The double leaves cash for a lodge; the
   quad survives the holiday week. The quad is right, and the game should
   make the player feel the queue if they cheap out.
3. **Ticket price.** The price stepper now matters: too high and the lot
   is empty on weekdays; too low and the queue cap does the rationing for
   you, badly. There is a sweet spot and it moves with the rating.
4. **Cat before patrol, or patrol before cat.** Both are needed; there is
   money for one before the holidays. The right answer depends on how
   steep the blue is.

**How it ends.** The base melts out in April. The player closes, and the
season report card shows a small operating profit, a rating around 0.6,
and a dominant complaint ("nowhere to eat" or "line too long"). The
player has learned the loop: crowd → queue → rating → crowd. Season-pass
sales at closing give a cash bump. The player pauses, builds, and skips
to the first freeze.

### Season 2 — Finding the Crowd

*"Give them a reason to stay all day."*

**What gets built.** With no guests on the mountain and no revenue
coming in, the closed months are when the player reshapes the base. They
paint a **lodge shell** — a footprint of cells near the lift base, an
L if they like — pick a storey count and a style, and the building
resolves itself: walls, corners, gables, and windows fall out of the
footprint's shape, with a shuffle button if the first roll isn't right.
They mark two perimeter cells as **doors** facing the path, and fill the
interior with a **rental counter** and a **cafeteria**; the cafeteria's
wall becomes glazed, the rental's door becomes a wide one. A **patrol
hut** goes up near the top station so response times are
short. A **bar** appears by the base for the afternoon crowd. The path
network grows: parking → rental → tickets → lift, with a **ski rack**
where the path meets the snow so guests aren't carrying skis through the
lodge.

Mid-season, with revenue up, the player buys the **mid-mountain bowl
parcel** and builds a **second fixed quad** into it. The bowl gets two
blues and the first **black**. The trunk lift is now a feeder; the queue
problem moves uphill.

**What pushes back.**

- *Door queues.* The rental counter is the new bottleneck: everyone
  arriving at 9 a.m. needs skis at the same time. One counter with one
  staffer is a 20-minute line. The player learns to add a second counter
  module or a second staffer — the first staffing decision.
- *Walk complaints.* If the lodge went up in the wrong place, the
  event feed says so: "guests complaining about the walk from parking."
  Moving a lodge is expensive; this is where the base-triangle decision
  from Season 1 pays off or doesn't.
- *Lift wear.* The trunk quad has run every day for a season and a half.
  Its wear crosses the line and one morning it doesn't start. Guests who
  are already on the mountain are stuck in the bowl. A maintenance
  contract (a shed upgrade) prevents this; the player learns that lesson
  once.
- *Expert guests start arriving* because a black exists, and they are
  demanding: they ride fast, they want the powder, and they are the ones
  who will pay for the gondola later. Right now they find one black and
  leave bored.
- *The loan.* Interest is due; the player either paid it down over the
  summer or it is eating the weekday margin.

**Key decisions.**

1. **Lodge footprint and door placement.** A big shell with one door is a
   worse building than a small shell with two. The order of modules along
   the path (rental before tickets before lift) is the puzzle. The shape
   is the player's; the architecture is the game's.
2. **Buy the bowl, or upgrade the trunk?** Upgrading the quad to a
   high-speed quad is unlocked once the rating clears 0.65. The bowl adds
   terrain and a new guest segment; the HS upgrade fixes the queue. The
   happy path buys the bowl first: terrain grows the crowd that pays for
   the upgrade.
3. **Pass pricing.** With a cafeteria and a second lift, the season pass
   is worth buying for far more guests. Price it too high and the ticket
   office sells nothing; too low and the player gives away the peak days.
4. **Staffing levels** for the rental, cafeteria, and bar against the
   weekday/weekend swing. Overstaff and the weekday P&L bleeds; understaff
   and Saturday's exit thoughts are all about lines.

**How it ends.** Rating is 0.7. Weekend days push 250 guests. The report
card's headline is "black terrain is popular but there's only one run."
The HS quad upgrade is unlocked and affordable.

### Season 3 — High Speed

*"Move the crowd faster than it arrives."*

**What gets built.** The trunk lift becomes a **high-speed quad** — same
line, new terminals, half the ride time, double the throughput. The
queue at the base evaporates and the bowl lift is now the choke point;
it gets a **second lane** and, late in the season, its own HS upgrade.
**Snow guns** go in along the green so the resort opens on the first
cold night rather than the first storm — weeks earlier than last year —
and so the base under the green is deep enough to outlast the spring.
The player buys the **treed ridge parcel** and cuts glades and two more
blacks into it, served by a third lift, and the resort now has three
pods and a real trail map.

**What pushes back.**

- *Merge zones.* Three pods funnel into one base. The last 200 metres of
  the green, the bowl's blue run-out, and the ridge's return trail cross
  each other above the lodge. Collisions and falls spike in the merge,
  and the beginners — who are the majority — are the ones getting hit. A
  **slow zone** painted over the merge and a re-routed return trail fix
  it; ignoring it caps the rating.
- *Early-season thin cover.* Without snowmaking, a warm November means
  the base is bare and the lifts sit on hold while the resort bleeds
  fixed costs. With it, the guns run on the cold nights and the green
  opens weeks before natural snow arrives. The guns' operating cost is
  real; running them in February on top of a metre of natural base is a
  waste.
- *Spring melt.* In April the lower mountain turns to slush and then to
  mud. The base run-outs go bare, guests boot-pack the last stretch, and
  the crowd thins out. The guns can't run in April — it's too warm — but
  the base they built in December and January is what decides whether
  the green survives into May. The player learns that snowmaking is a
  winter investment with a spring payoff, and that there is a day when
  staying open costs more than it earns.
- *Avalanche risk on the ridge.* The new parcel has a steep, wind-loaded
  face above the glades. After the first big storm the risk overlay goes
  red. Without patrol coverage of the ridge and a closure, an avalanche
  runs through a black with guests on it. The patrol hut's second unit
  and the ability to **close a trail** for the day are the answer.
- *Expert appetite.* Three blacks and glades bring the expert segment
  in properly. They fill the black runs, they buy passes, and the exit
  thoughts start saying "I've skied everything" — the signal that the
  mountain is out of terrain.

**Key decisions.**

1. **Which lift goes high-speed first.** The trunk (moves everyone) or
   the bowl (where the queue is now). The trunk is right because every
   guest rides it; the game should make the alternative feel wrong
   through the queue chart.
2. **Where the return trails cross.** The first time the player has to
   think about flow *on the snow* rather than in the base area: routing
   three pods into one base without a merge disaster.
3. **Snowmaking coverage, and when to open and close.** Guns are cheap
   individually but cover little. The player decides which trail must
   open first (the green), how deep a base to bank for spring, and — at
   the other end — on which day the shrinking crowd stops covering the
   attendants and the cats. Season length is now a lever.
4. **Ridge or no ridge.** The ridge parcel is the most expensive terrain
   yet and it comes with avalanche exposure. Buying it means committing
   to patrol coverage and closures — an operating cost, not just a
   capital one.

**How it ends.** Rating 0.75 sustained. 400 guests on a Saturday. The
report card says the resort is at terrain capacity: the bowl and the
ridge are full by 11 a.m., and the black trails are tracked out by
lunch. Across the gap — visible from the ridge — is the alpine parcel,
and the gondola is unlocked.

### Seasons 4–5 — The Gondola

*"Connect the mountain."*

**What gets built.** The gondola is a savings goal, not a purchase. It
spans the gap from the ridge to the alpine face — 2 km or more, crossing
terrain no chair can be built on. It costs several seasons of profit
plus the full credit line, and needs the alpine parcel bought and
patrolled before it is worth turning on. Its top station sits at the
highest point on the map; from there the black and double-black
terrain of the alpine face runs back down to a **second base pod** on
the far side, with its own small lodge, its own patrol post, and its
own HS quad to lap the alpine. The gondola carries skiers both ways:
the two pods become one resort.

**What pushes back.**

- *Financing.* Even at Season 3 profitability the gondola needs the full
  credit line plus a season of savings. The moment it's bought the
  player is maximally leveraged, and one bad season — a low-snow year
  where the resort opens six weeks late — hurts in a way nothing earlier
  did.
- *A late winter.* The alpine pod is high and cold and would open early;
  but the gondola's base station is in the valley, and if the valley is
  bare the gondola can't load. Snowmaking at the gondola base is what
  makes the alpine reachable in November.
- *Wind.* The alpine is exposed. On wind days the chairs on the far side
  go on hold; the gondola, being enclosed, runs. The player learns that
  the gondola is also weather insurance, and that the alpine pod without
  the gondola would be closed a third of the season.
- *Two-pod flow.* Guests who ride over to the alpine in the morning have
  to come back. At 3 p.m. the gondola's far-side queue is the longest
  line on the mountain. Ride time is fixed by span; the only lever is
  cabin count, and the only mitigation is giving guests reasons to stay
  on the far side longer (the second lodge, the bar).
- *Avalanche, for real.* The alpine face is the most dangerous terrain
  on the map. Patrol needs a post up top, closures need to be routine,
  and a slide that catches guests here can end a season's rating.

**Key decisions.**

1. **Where the gondola lands.** The bottom station on the ridge pod
   versus the base: the ridge is shorter and cheaper but forces every
   alpine guest to ride two lifts first; the base is the true
   peak-to-peak and costs a third more. The happy path lands it at the
   base — it's the reason the gondola exists.
2. **When to pull the trigger.** The day cash plus credit line clears
   the price, or a season later with a buffer against a bad winter.
   Buying mid-season means it earns immediately; buying at the limit
   means one dry December is a crisis. The report card and the forecast
   strip are the inputs.
3. **The alpine pod's size.** A full second base area, or just a warming
   hut and a lift? The full pod earns more and holds guests on the far
   side longer, but it doubles the staffing bill.
4. **Whether to add heli.** With the alpine open and expert guests in
   volume, heli-skiing becomes a premium add-on. It's optional in the
   happy path; the gondola is the goal.

**How it ends.** The gondola opens. Guests ride over the gap. The rating
crosses 0.8 and stays there. The report card at the end of Season 5 is
the campaign's summary: five seasons, three high-speed chairs, one
gondola, two base pods, a mountain that beginners and experts both call
their own. The player can keep going — there's a fourth parcel, there's
heli, there's the hotel — but the arc is complete.

---

## 3. Buildings and Infrastructure, in Order of Need

| Season | Build | Why it's needed then |
|---|---|---|
| 1 | Parking lot | Guests have to arrive somewhere |
| 1 | Pedestrian path | Lot → ticket → lift; where the crowd becomes visible |
| 1 | Ticket office | Day tickets are the revenue; passes at season end |
| 1 | Fixed-grip quad (trunk lift) | The first lift; sited to serve green + blue |
| 1 | Green + blue trails | Without a green, beginners don't come; without a blue, nobody stays |
| 1 | Equipment shed + snowcat | First storm buries the groom |
| 1 | Patrol hut | First injury with no help is the worst rating hit |
| 2 | Lodge shell (painted footprint, resolved exterior) + rental counter + cafeteria | Session length; the rental queue is the first staffing lesson |
| 2 | Ski racks | Guests de-ski at the path edge; visible clustering |
| 2 | Bar | Thirst; afternoon revenue |
| 2 | Mid-mountain parcel + second fixed quad | Terrain growth; first black |
| 2 | Maintenance contract (shed upgrade) | Trunk lift wear |
| 3 | HS quad upgrade (trunk) | Throughput; unlocked by rating |
| 3 | Dual lanes on the bowl lift | Queue management |
| 3 | Snow guns on the green | Open on the first cold night, not the first storm; bank a base for spring |
| 3 | Ridge parcel + third lift + glades + blacks | Expert segment; avalanche exposure |
| 3 | Slow zone | Merge-zone safety |
| 3 | Trail closure | Avalanche days |
| 3 | Second patrol unit | Ridge coverage |
| 4–5 | Alpine parcel | Gondola destination |
| 4–5 | Gondola | The savings goal |
| 4–5 | Second base pod: small lodge, patrol post, HS quad | Far-side dwell time |
| 5+ | Heli, hotel, more parcels | Beyond the arc |

---

## 4. What Pushes Back — The Systems That Create Challenge

These are the forces the player is managing against. Most already exist in
the simulation; the ones marked *new* are what this iteration adds so the
story above actually happens.

| Force | Exists today | What changes |
|---|---|---|
| Weekend / holiday demand swing | *new* | Demand multiplier by weekday and holiday week |
| Weather → demand and mood | *new* | Powder-day rush, rain-day emptiness, storm-day satisfaction swings |
| Storm buries grooming | yes | Unchanged; now has a demand payoff the next day |
| Queue cap → walk-aways | yes | Now surfaced in the event feed and the rating breakdown |
| Injury without patrol | yes | Rating hit is already large; needs to be *visible* |
| Thirst / hunger → early departure | yes | Cafeteria module restores hunger (today nothing does) |
| Boot-walk patience drain | yes | Becomes the lever that makes base layout matter |
| Door queues at buildings | *new* | Capacity + service rate + staff per module |
| Lift wear and breakdown | *new* | Wear per ride; breakdown → OnHold; maintenance contract |
| Avalanche | yes | Adds trail closure as the mitigation and patrol coverage as the requirement |
| Merge-zone collisions | partial | Skier-vs-skier hazard exists in steering; needs a fall/injury consequence and a slow-zone tool |
| Season length is weather-defined | partial | Lifts already hold on bare base; demand must collapse when nothing is open; calendar runs through summer instead of skipping it; skip-to-date / skip-to-condition control |
| Early-season thin cover | yes | Snow guns exist; now they decide opening day |
| Spring melt | yes | Now decides closing day; a deep man-made base extends the season |
| Cash flow, interest | *new* | Credit line, interest charged monthly, bankruptcy floor |
| Expert boredom | partial | Explore goal exists; needs terrain-exhaustion thought and demand feedback |
| Wind holds on exposed lifts | partial | Wind exists in weather; chairs should hold, gondola should run |

---

## 5. The Decisions the Player Has to Get Right

In rough order of consequence:

1. **The base triangle.** Parking, tickets, lift base within a short
   walk, with a green run-out that reaches it. Wrong here is expensive to
   fix for the rest of the game.
2. **Lift siting for terrain mix.** A top station that serves only one
   difficulty is a lift for one guest segment.
3. **Ticket and pass pricing.** The first elastic lever; moves with
   rating and terrain.
4. **Order of investment.** Cat before patrol, bowl before HS upgrade,
   trunk before bowl for high-speed, ridge before gondola. The happy path
   has an order; the game should let the player deviate and feel why.
5. **Lodge footprint, module mix, and door placement.** The base-area
   layout puzzle.
6. **Staffing against the weekly swing.** Overstaff weekdays or
   understaff weekends.
7. **Return-trail routing across pods.** The first on-snow flow problem.
8. **Snowmaking coverage and season length.** Which trail opens first,
   how deep a base to bank, and which day to close.
9. **Parcel purchases.** Terrain versus cash versus exposure.
10. **Gondola landing and leverage.** The capstone.

---

## 6. Timeline From the Resort's Point of View

| When | The resort |
|---|---|
| **S1 October** | Paused on a bare mountain, one parcel. Base triangle laid out, trunk quad, shed with one cat. Credit line partly drawn. Skip to first snowfall. |
| **S1 first storm** | Opens with one lift, a green and a blue — whenever the weather delivers. Weekday crowds tiny. Loss-making. |
| **S1 Christmas** | First crush. Queue cap hit daily. Rating dips. Player adds a lane or eats the complaints. |
| **S1 Jan–Feb** | Storms. Grooming becomes routine. First serious injury; patrol hut goes up if it wasn't there. Holiday weekend (Presidents' Day) is the season's peak day. |
| **S1 April** | Base melts out. Crowd thins to nothing. Player closes; report card: small profit, rating ~0.6, "nowhere to eat." Season-pass sales at close. |
| **S1 closed months** | Paused. Lodge shell drawn, rental + cafeteria placed, paths extended, ski racks. Loan partially paid. Skip to first freeze. |
| **S2 first snow** | Opens with a lodge. Rental queue is the new problem. Staffing lever discovered. |
| **S2 Jan** | Bowl parcel bought, second quad built the same day. |
| **S2 Feb–Mar** | Bowl opens with two blues and a black. Experts start showing up. Trunk lift breaks down once. Maintenance contract bought. |
| **S2 close** | Rating ~0.7. HS upgrade unlocked. Pass sales strong. Report card: "black terrain popular, only one run." |
| **S2 closed months** | Trunk quad → HS quad. Snow guns along the green. Skip to first freezing night. |
| **S3 first cold night** | Guns run; the green opens on man-made snow weeks before the first storm. HS trunk empties the base queue; bowl lift is the choke. |
| **S3 Dec–Jan** | Ridge parcel bought; third lift and glades cut; slow zone painted at the base merge after a bad week of collisions. |
| **S3 Feb** | First red avalanche day on the ridge. Closure. Second patrol unit. |
| **S3 May** | Man-made base carries the green into May; the upper mountain closed weeks ago. Rating 0.75. 400 on a Saturday. Terrain at capacity. Gondola unlocked. Report card: "skied out by lunch." |
| **S3 closed months** | Alpine parcel bought. Bowl lift goes HS. Saving. |
| **S4** | Profitable, still short of the gondola. A dry December opens the resort six weeks late and shows what leverage would feel like. Player banks the buffer. |
| **S4 March** | Cash plus credit line clears the price. Gondola bought; it turns the next morning. Far-side pod follows: lodge, patrol post, HS quad. Guns at the gondola base. |
| **S5 first cold night** | Gondola base opens on man-made snow; the alpine is already deep. Two pods, one resort. |
| **S5** | Wind days prove the gondola's worth. 3 p.m. far-side queue becomes the new routine problem. Rating crosses 0.8. |
| **S5 close** | Campaign report card. Arc complete. |

---

## 7. Reference Economy — Shape, Not Values

The current constants cannot produce this arc; per-ride tickets at $10
against six-figure builds mean no growth curve. The reshaped economy has
these properties:

- **Revenue is per visit, not per ride.** A day ticket charged at
  arrival. Per-ride pricing survives only for heli.
- **A healthy pod pays for itself in about a season.** One lift with its
  trails, at a reasonable price and rating, nets roughly its own build
  cost over 186 days. The second pod is affordable in Season 2 with the
  credit line; the HS upgrade in Season 3 from savings; the gondola in
  Season 4 only with the full line and two seasons of profit.
- **Opex is 30–50% of gross at a well-run resort.** Attendants, cats,
  patrol, guns, module staff. Enough that overbuilding hurts, not so
  much that growth stalls.
- **Price is elastic.** Arrival probability falls as price rises
  relative to a guest's willingness (skill- and rating-dependent). There
  is a revenue-maximising price and it moves.
- **Passes are affordable to the middle of the pool** at default price,
  not just the top decile.
- **The gondola costs 6–10× a high-speed quad** and 2–3× the credit line.
  It is a multi-season savings goal by design.
- **Interest accrues daily and is charged monthly**, including through
  the closed months — idle months cost money, which is why snowmaking
  and a long season matter. Cash below the credit floor for thirty
  consecutive days is bankruptcy.
- **Closed months are cheap but not free.** Standby costs (cats parked,
  no attendants) run while nothing is open; the player is not punished
  for skipping, but a resort that can open in November and close in May
  out-earns one that runs January to March on the same capital.

Indicative magnitudes, for a starting cash of $1M and a $1M credit line:
parking $150k · ticket office $80k · fixed quad $700k + per metre · HS
quad $1.5M + per metre · lodge shell $100k + per cell · module $50–150k ·
shed $200k, cat $150k · patrol hut $120k · snow gun $40k · gondola $4M +
per metre (a 2.5 km span lands near $10M) · day ticket $60–90 · pass
$400–700 · a Season 1 weekday 80–150 guests, weekend 250–350.

---

## 8. Systems This Story Requires

The story above depends on the following being true. Existing systems are
listed so the dependency is explicit; new ones are what gets built next.

**Existing, load-bearing.** Guest catchment and demand poll · GOAP
goals · ski physics and falls · trails and terrain match · lift types and
upgrade path · dual lanes · snowcats and grooming · weather chain and snow
kinds · avalanche · patrol rescue · parcels · season calendar · history
and charts · ticket office and passes · bar.

**New — economy.** Day-ticket revenue at arrival · price elasticity in the
demand poll · credit line with monthly interest and a bankruptcy floor ·
rebalanced build and operating costs · standby costs while closed · pass
pricing that reaches the middle of the pool.

**New — calendar and season.** Calendar runs through the whole year (no
skipped off-season) · resort open/closed state derived from whether any
lift has snow at its base · demand collapses to zero when nothing is
open · a "season" derived from first-open to last-open day, driving the
report card, pass expiry, and `VisitsThisSeason` reset · skip control:
fast-forward to a date or to a condition (first freezing night, first
snowfall, base depth threshold) · snowmaking as the lever on both ends
of the season.

**New — base area.** Pedestrian paths as painted cells · buildings with
multi-cell footprints, real passability, and doors · lodge shells painted
as cell footprints with procedurally resolved exteriors (below) · interior
modules (rental, cafeteria, bar, lockers, ski school) with capacity,
service rate, staff, and price · door queues · ski racks · hunger restored
by the cafeteria.

**Shell resolution.** The Townscaper / Tiny Glade principle: the player
authors the footprint and a few coarse choices — storeys, style palette,
which perimeter cells are doors — and the building resolves itself from
that shape. Exteriors are a tile kit (wall, corner, inside corner, door,
glazed wall, gable, hip, ridge) authored parametrically in OpenSCAD and
compiled through the existing `scad2obj` pipeline, so door tiles publish
their entrance anchor via `MOGUL_META` exactly as parking publishes its
driveways. Tiles are chosen by neighbour configuration (marching-squares
over the footprint, at half-cell resolution so a 20 m lodge is eight
tiles wide), which never fails and resolves instantly; a seed picks among
variants for window rhythm and trim, and a shuffle button rerolls it.
Wave function collapse is the upgrade path if facades later need
continuity constraints the neighbour lookup can't express. Interior
modules bias the facade — a cafeteria glazes its wall, a rental widens
its door — so what a building does shows on the outside. The shell
renders as instanced static tiles through the existing batch; the apron
pass already grades the ground beneath it. L-shapes and courtyards come
free from painting cells. The result: every resort's base area looks
different and the player feels they designed it, with a dozen tile
meshes instead of a thousand assets.

**New — mountain operations.** Slow zones · trail closures · lift wear,
breakdowns, and a maintenance contract · wind holds on chairs with the
gondola exempt · gondola-specific rules (long span, few towers, crosses
gaps chairs cannot).

**New — feedback and structure.** Event feed with click-to-jump ·
positive satisfaction events (great run, first ride, walked right on,
powder day) · weekend and holiday demand cycle · weather → demand and mood
· season report card at derived season close · scenario milestones
that double as unlock gates (fixed quad → HS → gondola → heli) ·
terrain-exhaustion thought for experts.

---

## 9. Out of Scope for This Iteration

Hotels, condos, and real estate · train station · heli beyond what already
exists · snowboard physics · wall-by-wall building construction ·
construction time on buildings or lifts · night skiing and lit runs · summer operations
(hiking, biking, sightseeing rides) · audio · modding. All remain in
DESIGN.md or are deliberately excluded; none are needed for the arc above.

## Game Design Document: Mountain Mogul (Working Title)
**Project Vision:** A high-fidelity ski resort tycoon that blends real-world geography with deep economic simulation.

---

### 1. Executive Summary
**Mountain Mogul** is a management simulation where players design, build, and operate world-class ski destinations. *Mountain Mogul* leverages real-world geospatial data to allow players to recreate their local "hidden gems" or massive international peaks. The core loop balances technical mountain operations with a complex economy.

---

### 2. Core Pillars
*   **Geospatial Authenticity:** If it exists in the real world, you can ski it. The terrain isn't just a map; it’s a canvas based on actual topography.
*   **Living Mountain Simulation:** Thousands of individual "Agentic Skiers" navigate the mountain based on their skill, fatigue, and personal goals.
*   **Vertical Integration:** Players don’t just manage lift tickets; they manage the entire mountain ecosystem, from the parking lot to the luxury penthouse.
*   **The "Eternal Winter":** A 24/7 operational cycle that prioritizes constant flow and management over time-of-day transitions.

---

### 3. Key Gameplay Mechanics

#### A. Terrain & Biomes
The game utilizes a **Geo-Data Import Tool** to fetch real-world heightmaps. Players must navigate three distinct biomes that affect construction costs and skiing difficulty:
*   **Forested:** Dense trees; requires "glading" (clearing) for runs.
*   **Sub-Alpine:** Mixed terrain; ideal for intermediate lodges.
*   **Alpine:** High altitude, rocky, and steep; high maintenance but attracts expert skiers.

#### B. Transport & Uplift
Movement is the heartbeat of the resort. Players must strategically place a hierarchy of lifts:
*   **Surface Lifts:** Magic carpets and tow ropes for beginners.
*   **Aerial Lifts:** Fixed-grip chairs, high-speed detachables, and massive gondolas.
*   **Specialty Transport:** CAT skiing for backcountry access and Heliskiing pads for the ultra-wealthy "whale" guests.

#### C. The "Agentic Skier" System
Each guest is a unique agent. See [SKIER_AI.md](SKIER_AI.md) for the full AI architecture (L0 GOAP planner + L1–L3 continuous controller).

**Per-skier state:**
*   **Energy:** Session-level fatigue budget. Drains while skiing, recovers on lifts and at lodges.
*   **Hunger:** Grows over time; resolved by eating at a lodge.
*   **Fun:** Composite satisfaction signal (`Mood` in the AI doc). Modulated by run quality, queue waits, falls, and matched-difficulty terrain.
*   **Revenue:** Cumulative spend per skier — lift ticket, food, lodging, merch — rolled into resort revenue on departure.
*   **Injury:** Boolean event state; triggered by overshoot, ungroomed terrain at low skill, or unsafe density. Pulls the skier out of play and dispatches Ski Patrol.

**Per-skier traits:**
*   **Skill Level:** Beginner, Intermediate, Expert, Pro. Determines run selection and sets the agent's base fatigue accumulation rate on a given run.
*   **Goals:** "Hunt powder," "Find the shortest line," "Go to après-ski," or "Stay near the lodge."
*   **Satisfaction:** Composite score driven by lift wait times, grooming quality, trail crowding, and whether runs at their skill level are available and reachable. An expert who can only access beginner terrain, or a beginner who can't find a safe way down, both tank their satisfaction.
*   **Fatigue & Injury:** Each agent tracks a fatigue value that rises while skiing and falls while resting (lift queue, lodge). Fatigue accumulates faster on runs above the agent's skill level. Once fatigue crosses an injury threshold, each successive turn carries a probability of an injury event — which removes the agent from play and dispatches Ski Patrol.

#### D. Mountain Operations (The "Hidden" Game)
Efficiency is determined by behind-the-scenes infrastructure:
*   **Grooming:** Deploy a fleet of snowcats to maintain "corduroy" snow. Ungroomed trails turn into moguls, increasing difficulty and injury risk.
*   **Ski Patrol:** Quick response times to injuries keep the mountain open and reputation high.
*   **Maintenance:** Lifts require parts and labor. A breakdown on a primary artery can tank your resort's rating instantly.

---

### 4. Park Rating

A single top-bar metric that summarizes how the resort is doing. Computed as an exponential moving average of skier `Fun` at departure (see [SKIER_AI.md](SKIER_AI.md) §Resort rating).

Park Rating is the player's primary feedback signal:
*   Drives the spawn rate at parking — bad rating means fewer arrivals.
*   Drives real-estate demand — condo / hotel sales soften when rating drops.
*   Surfaces top complaint reasons so the player can diagnose (long queues, mismatched difficulty, falls, parking full).

Specific events that move the rating disproportionately: injuries, deaths, parking turn-aways, and long-line cascades across the whole resort.

---

### 5. Economy & Real Estate
Growth is funded by more than just lift tickets. The game features a **Real Estate Economy**:
*   **Land Acquisition:** Take out high-interest loans to expand your boundary.
*   **Zoning:** Develop base areas with parking, hotels, luxury condos, and retail.
*   **Hospitality:** Manage restaurants and bars to capture "off-mountain" revenue.

---

### 6. Building Catalog

The full set of placeable buildings and zones, grouped by role. Most exist to either move skiers, keep them happy, keep them safe, or extract revenue.

#### A. Uplift
| Building | Role |
|---|---|
| **Lifts** | A variety of lifts get skiers up the mountain for more fun. Standing in line drains fun. Surface lifts, fixed-grip chairs, high-speed detachables, gondolas — each with different throughput, cost, and skill suitability. |

#### B. Hospitality (on-mountain revenue & rest)
| Building | Role |
|---|---|
| **Lodges** | Provide food and rest for hungry and tired skiers. |
| **Shops / Restaurants / Bars** | Give skiers something to do while resting and provide off-mountain revenue. |
| **Ice Rink** | Non-ski attraction; gives lodgers something to do. |
| **Sledding Hill** | Non-ski attraction; family-friendly. |

#### C. Terrain Operations
| Building / Zone | Role |
|---|---|
| **Cat Shed** | Houses a fleet of snowcats. Snowcats groom terrain and cut cat trails. |
| **Grooming** | Most skiers prefer groomed terrain, and it's easier for beginners — fewer falls and injuries. |
| **Moguls** | Some skiers actually prefer moguls. An expert bombing moguls can provide fun for skiers watching from a lift. |
| **Cat Trails** | Give beginners an easy way down the mountain. Cat trails can fill up, increasing injury risk. |
| **Slow Zone** | Encourages skiers to go slow through congested areas, reducing injuries. |
| **Ski Area Boundary** | Keeps skiers off dangerous unpatrolled terrain. Injuries or deaths outside patrol coverage tank the rating hard. |

#### D. Safety
| Building | Role |
|---|---|
| **Ski Patrol** | Enforces slow zones and responds to injuries. |
| **Clinic** | Treats injuries on-site so skiers don't get mad and hurt the rating. |
| **Medevac** | Emergency air evacuation for serious incidents. |

#### E. Real Estate (long-term revenue & guest retention)
| Building | Role |
|---|---|
| **Condos** | Selling condos gives a quick burst of income when demand is there. Owners become a reliable source of skiers. |
| **Hotels** | Expensive to build but provide ongoing revenue and keep skiers on the mountain across days. |
| **Houses** | Similar to condos with even more income, but take up valuable land. |

#### F. Access
| Building | Role |
|---|---|
| **Parking** | Primary spawn point. Capacity caps daily attendance; a full lot turns guests away and dings the rating. |
| **Train Station** | Alternative arrival mode. Higher throughput, no parking footprint. |

#### G. Premium / "Whale" Experiences
| Building | Role |
|---|---|
| **Heli Pad** | Enables Heli Skiing. |
| **Cat Skiing** | Snowcat-assisted backcountry access for advanced skiers. |
| **Heli Skiing** | Top-tier experience for ultra-wealthy guests. High revenue per skier. |

---

### Easter Eggs
These should be "discoverable" by the player and not placeable, they're things that just happen if the conditions are right
* Pond skimming — warm temps + a flat run-out + low-snow patch turns into a puddle; an expert with full Fun yolos across it. Crowd gathers, Fun bonus radiates to nearby skiers.
* Gaper Day — last weekend of season, a fraction of skiers spawn in absurd retro outfits. Pure cosmetic, nostalgia bonus to Park Rating.
* Send It — Pro encounters a small cliff feature on steep terrain, hucks it. Possible "stomp" (big Fun + audience bonus) or "yard sale" (gear scattered, Ski Patrol response).
* Bootpacking — a Pro at full energy hikes above the highest lift to access an untouched alpine face. Visible little figure on the ridge; pulls other Pros toward that terrain.
* Mogul Bomber — an expert on a bumped-up run draws an audience from the adjacent lift (you mentioned this in the doc — formalize it as an emergent event).
* Yeti Sighting — extremely rare, only when alpine biome + low traffic + heavy snow; one skier returns claiming to have seen it. Tiny Park Rating wobble, big screenshot moment.

### Related Documents

*   [SKIER_AI.md](SKIER_AI.md) — agent AI architecture (planning + steering)
*   [SNOW.md](SNOW.md) — snow surface model and friction multipliers

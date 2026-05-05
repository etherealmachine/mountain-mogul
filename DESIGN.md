## Game Design Document: Mountain Mogul (Working Title)
**Project Vision:** A high-fidelity ski resort tycoon that blends real-world geography with deep economic simulation.

---

### 1. Executive Summary
**Mountain Mogul** is a low-poly management simulation where players design, build, and operate world-class ski destinations. Unlike traditional builders, *Mountain Mogul* leverages real-world geospatial data, allowing players to recreate their local "hidden gems" or massive international peaks. The core loop balances technical mountain operations with a complex real-estate economy.

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
Each guest is a unique agent with:
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

### 4. Economy & Real Estate
Growth is funded by more than just lift tickets. The game features a **Real Estate Economy**:
*   **Land Acquisition:** Take out high-interest loans to expand your boundary.
*   **Zoning:** Develop base areas with parking, hotels, luxury condos, and retail.
*   **Hospitality:** Manage restaurants and bars to capture "off-mountain" revenue.

---

### 5. Technical & Visual Style
*   **Visuals:** Clean, low-poly aesthetic with a high-contrast color palette. This ensures that even with 5,000+ agents on screen, the game remains performant and readable.
*   **Time System:** No day/night cycle. This allows for a continuous "flow state" for the player, focusing on the constant pulse of the resort rather than scheduling shifts.

---

### 6. Development Roadmap (Phase 1 - MVP)
Three scenes - Start Menu, Scenario (main gameplay loop), and Scenario Editor

Start Menu has Start Game, Load Save, Scenario Editor, Settings, and Exit
Start Game transitions to Scenario with the Tutorial resort
Tutorial resort has a predefined terrain with trees and obstacles (tree stumps, rocks)
Scenario Editor loads the Tutorial scene and lets edit it and save it, buttons for placing trees and modifying terrain (menu bar style)
Scenario scene has a menu bar for removing trees, placing trees, building lifts, and building lodges
Lodges will spawn skiers, skiers will head towards the base of the lift, take the lift up, and ski down either into the lodge or to the lift base again.
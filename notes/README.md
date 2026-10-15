# Notes

All of the project's documentation. Open this folder as an Obsidian vault, then open the graph view.

Short cards describe each system and plan and link to each other. The long-form specs live in `specs/` (for example [[Guests Spec]], [[Snow Spec]], [[Vision]]); cards point at them instead of copying them. To hide the specs in the graph, filter it with `-path:specs`.

## Schema

Each card has YAML frontmatter:

- `title` — name shown in the graph
- `kind` — `concept`, `amenity`, `system`, `stat`, `engine`, `tooling`, `plan`, `scenario`, or `spec`
- `status` — `idea`, `planned`, `partial`, or `shipped` (specs leave it out)

The graph colors nodes by `status` (green shipped, amber partial, blue planned, gray idea).

Link cards with wikilinks (`[[Hunger]]`). The graph is built from those links, so a connection that matters should appear in the prose. Add a dated line under `## Log` when the status changes.

Planned work goes in [[Next Steps]], the ordered list of everything to do. It holds new features only; known bugs are in [[Bugs]], what's left over on a built feature is in that feature's note, and what changed goes in the [[Changelog]], a line a day. A step that needs a design gets its own `plan` note in `next/` (for example [[Stored Trees]]); the system card links to it and keeps describing how things work today.

Update the notes in the same change as the work, not afterwards. When a step ships, or the user calls a feature done, set the plan note's `status`, mark its steps done or left over, and add a `## Log` line; take it off [[Next Steps]] (Priority especially) or rewrite its line there; add the sentence to today's [[Changelog]] line. When the code comes to work differently from what a note says it built, fix the note. A stale Priority entry (Skier Collisions, still first and "in progress" after it was done) is what this rule is for.

When a planned card gets built, keep it short and point at the spec. Detailed behavior that the code already implements belongs in the matching spec in `specs/`.

## Map

[[Mountain Mogul]] is the pitch and pillars, and [[Next Steps]] is the plan.

[[Amenities]] are the base-area services. [[GOAP]] is how a guest chooses what to do. The stats it reads are [[Hunger]], [[Thirst]], [[Energy]], [[Patience]], and [[Moments]]. [[Guest Types]] covers skiers, snowboarders, and the types still to come.

The mountain: [[Weather]] drives the [[Snow]] pack, which [[Snowmaking]] and [[Grooming]] maintain and [[Avalanche]] strips away. [[Ski Patrol]] rescues the injured.

Guests on the hill: [[Skiing]] is the run itself, on [[Lifts]] and [[Trails]].

Running the resort: [[Demand]] brings guests in, [[Finance]] keeps the books, [[Calendar]] sets the day and season, and [[Parcels]], [[Terrain]], [[Trees]], and [[Parking and Roads]] are the ground everything sits on. [[Event Feed]] is what the player hears about. [[Lodge Shell]] is how service buildings are built, and [[Pathfinding]] is how guests walk between them.

[[Scenarios]] are the starting maps. [[Scenario Campaign]] (in `scenarios/`) lays out the planned run of real resorts, from [[Boreal]] through [[Kirkwood]], [[Killington]], [[Mad River Glen]], [[Alta]], [[Asahidake]], and [[Zermatt]] to [[Palisades Tahoe]].

Under the hood: [[Architecture]] lays out the packages, [[Scenes]] are the screens, and [[Rendering]], [[UI and HUD]], and [[Save Format]] are what they share. For building and checking the game: [[Model Pipeline]], [[Scenario Editor]], [[Terrain Import]], [[Debug Tools]], [[Testbeds]], and [[Sim Queries]].

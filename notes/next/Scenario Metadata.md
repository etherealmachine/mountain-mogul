---
title: Scenario Metadata
kind: plan
status: shipped
---

# Scenario Metadata

Give each of the [[Scenarios]] a real name and a description, so the picker reads like a menu of mountains instead of a list of file names. Listed in [[Next Steps]].

## Steps

1. **Fields in the save.** A scenario carries:
   - a display name, using the `name` field the [[Save Format]] already has but never writes
   - a description: a few sentences on the place, the challenge, and what's new
   - a location (resort and region, for example "Donner Pass, California")
   - a difficulty from 1 to 5, and an order, so the picker can sort the campaign

   File names become stable identifiers that the player never sees.
2. **Picker shows them.** Buttons show the display name. Choosing one opens a detail panel with the location, difficulty, and description, plus Play and Back, instead of starting the game on the first click.
3. **Editor sets them.** A Scenario details panel in the [[Scenario Editor]] edits the name, description, location, difficulty, and order. The description needs a multi-line text box; today's text input is one line.
4. **Boreal keeps its job.** `tutorial.save` keeps its file name and becomes "Boreal", marked as the tutorial and sorted first. See [[Boreal]].
5. **Saves remember their scenario.** A game started from a scenario keeps the scenario's name, so the save list can show which mountain each save is on.

Small and self-contained. [[Scenario Goals and Rules]] builds on the same fields.

## Open questions (resolved)

- **Preview image.** Not yet. The detail panel is text only; a terrain thumbnail can come with the campaign art.
- **Description in-game.** Deferred to the goals panel in [[Scenario Goals and Rules]], which needs an in-game place to reread the scenario anyway.

## How it shipped

- `world.ScenarioInfo` on `World.Scenario` holds the name, description, location, difficulty, order, and a tutorial flag. The save writes them first, before the terrain, so `save.ReadScenarioInfo` reads them without decoding the rest: 0.2 ms per file on the tutorial map. Old saves wrote the placeholder name "scenario", which loads as no name.
- The picker sorts the tutorial first, then by order, then unordered scenarios by name. In play, a pick opens a panel with the name, location, difficulty, and wrapped description, plus Play and Back (Enter plays, Escape goes back). The editor's picker opens the file straight away and shows the file name next to the display name.
- The editor's Details button (also in the escape menu) opens a Scenario details dialog. The description box is multi-line: it wraps, and Enter starts a new line. Tab moves between fields; OK applies and marks the scenario unsaved.
- `tutorial.save` is now "Boreal", the tutorial, order 1, difficulty 1. Re-saving it also moved it to the stored-tree format.
- Player saves keep the scenario info, and the save list shows each save's scenario name.

## Log

- 2026-10-01: Planned. Nothing implemented yet.
- 2026-10-02: Shipped all five steps.

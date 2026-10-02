---
title: Scenario Metadata
kind: plan
status: planned
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

## Open questions

- Does the picker want a preview image or a small terrain thumbnail per scenario?
- Should the description show again in-game, for example in the escape menu, so the player can reread the goals?

## Log

- 2026-10-01: Planned. Nothing implemented yet.

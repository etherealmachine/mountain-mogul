---
title: Saved Games
kind: plan
status: done
---

# Saved Games

A game you start is saved as itself: named after its scenario ("Boreal"), kept up to date without asking, and renamed when you like. Several scenarios can be on the go at once, each picked up where it was left; two games of the same scenario are possible but not the usual case. Part of [[Scenes]] and [[Save Format]]; *for* the [[Scenario Campaign]], where a player moves between scenarios.

## Why

Read on 2026-10-15:

- **Save always asks for a name**, pre-filled with a timestamp (`save.DefaultSaveName`: "save-2026-10-15-0930") the first time, then the last name used (`Scenario.saveName`). A new game has no name until the player types one, and the list fills with timestamps.
- **Nothing saves on its own.** Main Menu and Exit Game drop everything since the last Save; there's no autosave.
- **A save is written in place** (`os.WriteFile` in `saveWorld`): a crash or a full disk mid-write leaves a broken file and the game is gone.
- **The save list is a list of files**: name, scenario name, age, click to load (`scene/savelist.go`). No rename, no delete, no copy, nothing about how far the game has got.
- **The scenario picker doesn't know about games**: picking Boreal always starts a new one, even with a Boreal game in progress.
- What's already there to build on: a save records its scenario's name and file (`ScenarioData.Name`, `ScenarioFile`), read cheaply from the header (`save.ReadScenarioInfo`); Continue loads the newest save (`save.MostRecentSave`); the demo's saves are kept apart (`demo-saves`).

## The model

**A game is one save file.** Starting a scenario makes a game and names it after the scenario: "Boreal". A second game of the same scenario is "Boreal 2", then "Boreal 3". The name is the file name in `~/.mountain-mogul/saves/` (as today), so renaming a game renames its file.

**It saves itself.** The game is written:

- on its first frame, so it's in the list straight away;
- each midnight, as the day report comes up, so a crash costs at most a day;
- whenever the game scene closes: Main Menu, Load, Retry, Exit Game or closing the window (`Scenario.Destroy`).

The dev screenshot harness never autosaves (`Scenario.NoAutosave`): it may have loaded the player's latest game.

Save in the escape menu saves to the game's own file at once, with no prompt ("Saved Boreal"). Writes are atomic: to a temporary file beside it, then renamed over the old one. The previous version is kept as `<name>.save.bak` (a hard link, so it costs nothing), so a bad save can be undone by hand. Every save file, scenarios included, is now written this way (`writeAtomic`).

**Renaming.** In the escape menu (Rename, a prompt with the current name) and in the games list. A name already taken is refused ("There's already a game called Boreal"). Renaming writes nothing but the file name.

**The games list** (the start menu's Load Save, once Load Game), newest played first. Each row shows the game's name, its scenario when the name doesn't start with it, the date in the game ("Dec 14"), the star rating, won or lost when decided, and how long ago it was played. Clicking the row continues it; Rename, Copy ("Boreal copy", keeping the original's time so Continue doesn't jump to it) and Delete (asking first) sit beside it. Continue on the start menu loads the most recently played game, as before.

**Starting a scenario that already has a game.** The scenario picker marks a scenario with a game in progress ("In progress: Boreal, Dec 14 (and 1 more)", the most recently played) and offers Continue and New game; a click elsewhere on the card continues. New game makes "Boreal 2"; it doesn't overwrite. Retry, after a lost scenario, restarts the same game under its name, after asking.

**What a save carries for the list.** A game's header carries `game`: the game day's date, the stars and the outcome (`save.GameData`), read with the scenario's info before the cells (`readHeader`). Scenario files don't carry it. Older saves list without a date or rating until saved again.

**Unchanged.** The scenario editor's saves, Save as Scenario, and the demo keeping its saves apart. Old saves still load: they're files in the same folder with their old names, shown in the list like any game (no date or rating until saved again). Pre-release, nothing is migrated.

## Steps

1. Done: **A game has a name and saves itself.** New games named after the scenario ("Boreal", "Boreal 2"); saved on the first frame, each midnight, and when the scene closes; Save without a prompt ("Saved Boreal"); atomic writes with a `.bak`.
2. Done: **Rename.** In the escape menu (Rename..., under Save) and from the list, refusing a taken name.
3. Done: **The games list.** In-game date, rating and outcome in the save header; rows that continue, with Rename, Copy and Delete; Load Game renamed Games.
4. Done: **Scenario picker knows about games.** "In progress" on a scenario's card, Continue or New game; Retry restarts the same game under its name, after asking.

## Open questions

Defaults taken unless the user says otherwise:

- **Autosave each midnight** rather than every few real minutes: a game day is 43 minutes at normal speed, so this loses at most that. Could add a real-time autosave if that's too much.
- **The file name is the game's name.** Simpler than a separate display name in the header, and rename is instant. The catch: names can't hold `/` or start with a dot (`SanitizeSaveName`), which no one should miss.
- **One `.bak`, not a history** of autosaves.
- **Save as** (save under a new name and keep playing it) is left out: Copy in the list covers two copies of one game.

## Log

- 2026-10-15: Planned. The user: named saved games, mostly auto-named after the scenario, with renaming; aimed at playing several scenarios at once, and less often, several copies of one.
- 2026-10-15: Built, steps 1–4 (`save/games.go`, `scene/savelist.go`, the scenario picker's cards, the escape menu's Rename). Checked headless: names, the backup, refusing a taken name, rename, copy, delete and reloading; and screenshots of the games list and the picker (`-menu -menu-screen games|scenarios`). Not yet played through in the running build.
- 2026-10-15: The start menu's entry renamed Load Save (the user).

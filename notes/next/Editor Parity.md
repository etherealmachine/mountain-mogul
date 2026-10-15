---
title: Editor Parity
kind: plan
status: partial
---

# Editor Parity

Every tool the game has, in the scenario editor too (free there), so a scenario can be built with what a player builds; and a test run to watch the lifts and trains move while building. Extends [[Scenario Editor]]; touches [[Lifts]], [[Rail Network]], [[Rope Lines]], [[Ski Racks]].

## From the user (2026-10-14)

- The lift popup in the editor; generally, all the game's tools in the editor.
- Test the trains and lifts in the editor, to see how they look.

## Built

- **Lifts**: the Lifts menu is the game's (`liftMenu`: every type, with thumbnails); placing one checks its shortest and longest line and its steepest grade, as the game does. Picking a lift's station with no tool opens its popup (`scene/editor_lift.go`): name, type, length, chairs, speed, the heli's fare, whether it's open at the start, bubbles, lights, a change to any other chairlift on the upgrade ladder (either way, free: `world.RetypeLift`), and Delete.
- **Transport**: the railway (Rail Network), Ski Rack (placed and turned like a building) and Rope (the game's rope tool, free); Remove takes out ropes and track.
- **Test run**: the lifts and trains run empty from the editor's start (`sim.Preview`): every cable lift's chairs go round at its speed; each station with a service has a train in from its entry, stopping at its stations, and out, the next setting off once it's gone. The top bar's pause holds everything where it is, trains too, to look at; play and fast (8×) run them on. `-editor-test N` runs it N seconds ahead in an editor screenshot.

## Still to do

- Building popups: lodges' rooms and services, sheds (snowcats, patrol), as in the game.
- The trail popup's game rows: snowmaking, lights.
- Helicopters in the test run.
- The editor's bottom bar is full at 1280 px wide: a new button needs a submenu, or a wider screen.

## Log

- 2026-10-14: Planned and built: lift popup, every lift type, ski racks, ropes, the test run.
- 2026-10-14: The test run runs from the start; pause holds the trains still instead of clearing them.

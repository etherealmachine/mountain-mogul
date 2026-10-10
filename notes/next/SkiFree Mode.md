---
title: SkiFree Mode
kind: plan
status: idea
---

# SkiFree Mode

Take a skier and ski the resort yourself. In SimGolf, playing your own course was fun by itself and was the best way to feel whether a hole worked; this does the same for runs. The name is a nod to SkiFree, yeti included. Part of [[Skiing]]; the yeti ties in with the Yeti sighting under Easter eggs in [[Next Steps]].

## Why

The user, 2026-10-10: playing the golf was a big part of SimGolf's fun, so the resort should have a mode where you ski it. Everything the guests ski on is already simulated (pitch, snow underfoot, moguls, trees, falls), so the run should feel like the mountain the player built.

## Sketch

- **Drop in.** Pick a lift top (or ride the lift up) and take control of one skier. The game keeps running around you: guests, lifts, cats, weather.
- **Same physics as the guests.** Player input replaces the steering in `internal/sim/skiing.go`: turn left and right, tuck, skid to check speed, stop. Edge grip, balance, moguls, trees, and falls (yard sales included) work exactly as they do for guests, at an expert's skill or one you pick.
- **The resort reacts.** Guests dodge you the way they dodge each other; crash into one and they think about it. Ski Patrol comes if you get hurt.
- **The yeti.** Leave the boundary, or ski too far into the backcountry, and the yeti comes for you, like SkiFree.
- **Feedback for building.** Optionally show what a guest would think of the run you just took (too steep for a green here, icy there), so playing it is also a way to check it.

## Open questions

- Is it a mode chosen from the menu, a button in a running game, or both? Does the clock run, slow down, or pause the economy?
- Camera: chase camera behind the skier, or the classic top-down SkiFree view as an extra?
- Is there a persistent "you" (an owner or a resort pro with their own stats, like SimGolf's pro), or just a skier for the run?
- Anything to chase beyond fun: timed runs, race gates, slalom courses, or a ghost of your best run?
- Controls on keyboard, mouse, and a gamepad.

## Log

- 2026-10-10: Idea from the user, unranked.

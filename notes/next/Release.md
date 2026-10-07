---
title: Release
kind: plan
status: idea
---

# Release

How the game ships, from the user, 2026-10-07: a free demo, then early access, then DLC.

1. **Free Steam demo** ([[Demo]]): the [[Boreal]] tutorial and [[Kirkwood]], as a separate build of the game with the extras stripped out, and maybe the code obfuscated.
2. **Early access**: release what the game has, and build on it in public.
3. **DLC**: snowboarders and the other guest types ([[Guest Types]]), and likely more after.

## The demo build

A different build, not a mode of the full game:

- Stripped: the scenario editor, the other scenarios, debug and screenshot flags, testbeds, and anything the demo leaves out (snowmaking). Likely Go build tags (`demo`), so the code isn't in the binary at all rather than hidden.
- Maybe obfuscated: a Go obfuscator (garble is the usual one) for the demo binary. Not decided.
- Steam: store page, demo depot, and whatever Steamworks the demo needs.

## Open questions

- What early access ships with beyond the demo: the rest of the [[Scenario Campaign]], the editor, and which [[Services]].
- Whether the demo's saves carry over into the full game.

## Log

- 2026-10-07: Release path from the user: free Steam demo as its own stripped build, early access, guest types as DLC.
- 2026-10-07: Dropped the snowboard roll until the DLC ([[Guest Types]]).

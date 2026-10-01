---
title: Architecture
kind: engine
status: shipped
---

# Architecture

A Go game on GLFW and OpenGL 4.1. `main.go` opens the window, creates the app, and pushes the first of the [[Scenes]]. The gameplay scene owns a simulation and a renderer:

- `world` holds all persistent state and no game logic.
- `sim` runs every system against it each tick: [[GOAP]], [[Skiing]], [[Lifts]], [[Demand]], [[Weather]], and the rest.
- `render` draws the world read-only; see [[Rendering]].
- `ai` is a leaf package of the AI types stored on each guest, so `world` never imports `sim`.

[[Save Format]] serializes `world`. [[UI and HUD]] widgets draw through the renderer's UI pass. All gameplay randomness comes from one seeded generator in `internal/rng`, and [[Weather]] seeds each day from its date, which keeps [[Testbeds]] runs reproducible.

Spec: [[Architecture Spec]].

## Log

- 2026-10-01: Package layout as described in [[Architecture Spec]].

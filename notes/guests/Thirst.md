---
title: Thirst
kind: stat
status: shipped
---

# Thirst

A countdown on each guest, from full at 1 down to leaving at 0. [[GOAP]] reads it. Spawned at random in `[0.5, 1)`. Drains while skiing, faster with altitude and with exertion on terrain above the guest's skill.

Below 0.25 the `RelieveThirst` goal sends them to a [[Bar]]. Below 0.05 that goal is tried before `GoHome`. Below 0.15 they think about wanting a drink. A drink sets thirst back to 1.

Leaving because of thirst does not dock [[Satisfaction]].

Spec: [[Guests Spec]].

## Log

- 2026-10-01: Drain, the thirst goal, the thought, and the drink restore are in.

---
title: Snowmaking
kind: system
status: shipped
---

# Snowmaking

Snow guns are placed buildings, each switched on or off from its popup. When the air is at −2 °C or colder, an enabled gun lays dense machine snow into the season base of [[Snow]] within three cells (15 m). A cold twelve-hour night adds about 9 cm.

[[Weather]] decides when that is possible through the hourly temperature curve, so guns run on cold nights even when afternoons are above freezing. In the season story this is how the resort opens before natural snow and stays open after it melts; see [[Vision]].

Code: `internal/sim/snowgun.go`. Gun range and threshold: `internal/world/world.go`.

## Log

- 2026-10-01: Guns, the temperature gate, and base buildup are in.

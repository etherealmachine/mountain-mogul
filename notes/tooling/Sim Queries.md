---
title: Sim Queries
kind: tooling
status: shipped
---

# Sim Queries

A small SQL dialect over the live sim state. Tables are `cells`, `guests`, `cats`, `buildings`, `lifts`, and `patrollers`, built by reflecting each entity into a row. `WHERE` takes Go-style expressions, with `AND` and `OR` accepted too.

Two ways in: a query at the end of a headless [[Testbeds]] run, or a local HTTP server on port 6061 that answers queries against a running game, for example `curl -G localhost:6061/query --data-urlencode "sql=SELECT x,z,avy_snow FROM cells WHERE avy_snow > 0"`. Queries run on the game thread between frames, so they never see half-updated state.

Useful for checks the [[Debug Tools]] panels don't cover: the most unstable [[Avalanche]] cells, every guest below a [[Hunger]] threshold, lift line lengths.

Code: `internal/sim/query.go`, `queryserver.go`.

## Log

- 2026-10-01: Query tables, the end-of-run hook, and the live server are in.

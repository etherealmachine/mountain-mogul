---
title: Demand
kind: system
status: partial
---

# Demand

Where guests come from. A fixed catchment of guests, each with a skill and a daily budget, is polled every 30 sim-seconds. Each guest at home rolls to arrive based on the resort rating, whether there is terrain for their skill, how crowded the [[Lifts]] are, and whether the day ticket plus parking fits their budget. Arrivals follow the clock: a morning rush, nobody in the last hour, nobody while the resort is closed.

The resort rating is a slow running average of each departing guest's [[Satisfaction]]. A better rating draws more guests and lets the resort charge more before they balk.

A guest without a pass is turned away if no building sells [[Tickets]]. On arrival, guests pay for parking at the lot (see [[Parking and Roads]]), and the ticket price is set aside from their budget. That money, and their spending on [[Amenities]], lands in [[Finance]].

Not built yet: [[Weather]] affecting arrivals, per-lot weighting, and guests remembering their last visit.

Spec: [[Demand Spec]]. Code: `internal/sim/demand.go`.

## Log

- 2026-10-01: Catchment poll, rating, price response, arrival curve, ticket gate, and parking fee are in.

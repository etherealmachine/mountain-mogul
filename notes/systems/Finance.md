---
title: Finance
kind: system
status: partial
---

# Finance

Building is instant and costs cash. Revenue comes from day tickets and season passes ([[Tickets]]), heli fares ([[Lifts]]), parking ([[Parking and Roads]]), and food and drink ([[Food Court]], [[Bar]]). Each day charges operating costs if the resort opened at all and lower standby costs if it did not, broken down by category.

A credit line covers going negative: $1M by default at 12% a year, accrued daily and billed at month end. Thirty straight days below the credit floor is bankruptcy. Interest charges and the floor warnings go to the [[Event Feed]].

[[Vision]] still lists a full cost rebalance and staffing as a cost the player controls (see [[Rental Shop]]).

Code: `internal/sim/credit.go`. Revenue and cost categories: `internal/world/history.go`.

## Log

- 2026-10-01: Revenue by category, open and standby costs, the credit line, interest, and bankruptcy are in. Staffing and the cost rebalance are not.

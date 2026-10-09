---
title: Readable Complaints
kind: plan
status: idea
---

# Readable Complaints

Make every complaint say what to fix, and where. A review line should lead the player to the building, lift, or run behind it, and to the kind of fix: build more, build something new, build it closer, or charge less. Follows [[Star Ratings]] and [[Scoreless Rating]]; read in the Reviews tab and the guest popup ([[Moments]]).

## Why

From the user, 2026-10-09, after the Christmas check on the Boreal Goals Test ([[Season Calendar]]). The day's top complaint was "this place needs a lodge" (251 of 890 guests), with "nowhere to warm up" and "it's packed in there". The cause was one food court of 40 seats in a 4-tile unheated shed, with 41 guests waiting at its door at 3 pm. What the player could see:

- **The Reviews tab** points the right way (a lodge, warmth, crowding), but "this place needs a lodge" is wrong on its face: there is one, it's full and unheated. And the tab shows completed days only ("as of Dec 9"), so a holiday rush's problems appear the day after, when the rush is over.
- **The building popup** says "Shed, unheated" but nothing about how full it is.
- **The room popup** has the clearest signal ("40 / 40 seats, 41 waiting"), two clicks in, with the text running past the window's edge.
- **Nothing on the map or top bar** flags it.

The user: naming the building could help.

## Ideas

- **Name the place.** "Food court Shed is packed", "no seat at the Food court Shed", "the line at Lift 1 was too long". Moments already carry a context ID for some kinds (a nice place, a packed one, a long line); the review line and the Reviews tab group by it. Planner conditions (no lodge, no lounge) gain one: the place the guest tried.
- **Say why a need went unmet**, as separate moments:
  - **Full**: every seat taken, or the line too long ("no seat at the Food court Shed").
  - **Missing**: nothing offers it ("nowhere to warm up" when there's no lounge or heated building).
  - **Wrong kind**: it exists but can't serve it ("the Food court Shed is freezing", unheated).
  - **Too far**: the guest gave up on the way.
  - **Too dear**: priced out (already "everything here costs too much"), or out of money (a pass bought with the day's lunch money; fixed for passes 2026-10-09, but other spending can still do it).
- **Today so far** in the Reviews tab, beside the last full day, so a rush's problems show while it's on.
- **Occupancy in the building popup**: seats in use, the counter, the line at the door, per room, so the first click shows a full building.
- **Jump from a review line** to its place, as event rows already do.
- **A marker over a full building** (guests waiting at the door), like the pin over a fallen guest.

## Small bugs seen

- Daily costs show as "$$180/day" in the room and building popups.
- "40 / 40 seats, 41 waiting" runs past the room popup's edge.

## Log

- 2026-10-09: Written after the Christmas check; the user: complaints should be easier to understand, and naming the building could help.

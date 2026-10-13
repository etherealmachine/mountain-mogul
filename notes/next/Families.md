---
title: Families
kind: plan
status: planned
---

# Families

Children as guests, arriving and skiing with their parents, and the [[Ski School]] that teaches them. Builds on [[Groups]], which already brings guests in together, lets a leader plan for the weakest member, and has each member leave their own review. Part of [[Guest Types]]; touches [[Demand]], [[Moments]], [[Star Ratings]], [[Rendering]], and [[Services]].

## Decisions

From the user, 2026-10-13:

- **Base game, not DLC.** Families are core to a ski resort, and the Ski School and [[Childcare]] wait on them. Snowboarders stay DLC ([[Release]]).
- **Kids, family groups, and the Ski School** in the first version. Childcare comes after.
- **What's different about a kid:** smaller, and tires sooner. Their own tastes and falls stay as an adult's for now.
- **Parents' stars hinge on their kids.** A parent's day is only as good as their kids'.
- **Lessons for kids and beginner adults**, in the same school.
- **Instructors are staff who ski.** The school has a headcount of instructors at a wage; each one shows up as an expert skier in a school jacket and leads a class round the hill as a group leader.
- **Lessons last.** The skill a guest gains is kept in the catchment, so kids who learn at your resort come back skiing blues.
- Not in the [[Demo]]: the new code sits behind `!demo` like the other left-out features.

## Steps

In order. Each step builds with `go build` and `go vet`, and is checked headless on the Boreal Goals Test and on screen.

1. **Kids in the catchment.** An age on each guest (adult or child), saved. `GroupMix` gains a family share, and `formEntryGroups` (`world/group.go`) builds a family as one or two adults and one to three kids. Kids' skill rolls low (mostly beginners, some intermediates). A kid never leads (`joinParty` picks an adult), never drives, and never stays on alone: when the last adult heads home, the kids go too. The family share is in the editor's Guests tab with the rest of the group mix.
2. **Kids look and tire like kids.** A smaller figure (about two-thirds scale, in `place` in `render/figure.go`), with the seat, unload and carry heights adjusted so a kid sits on the chair rather than floating over it. Energy drains faster, so a family needs breaks sooner, and comfort speed sits lower.
3. **Parents' stars follow the kids.** A parent's star level is at most one above their worst-off kid's; a kid's highlight gives the parent one too ("the kids loved it"). Family moments: "waited for the kids at every turn", "the whole family on one chair". Kids' reviews count in the rating like anyone's.
4. **The Ski School building.** A service room in the [[Building Tool]] with a desk and a meeting spot on the snow outside, best near gentle terrain. The school popup sets the instructor headcount, the wage, and the lesson price. A lesson need is rolled per visit: kids whose parents want harder terrain than they do, and a share of beginner adults (the Ski School card's lesson need). Guests sign up at the desk and wait at the meeting spot for a class.
5. **Classes on the hill.** An instructor leads a class of up to eight as a group leader, on the existing follower code (`sim/groups.go`), planning at the class's level and keeping to the gentlest lift the school can reach (a chair is fine; a magic carpet once it exists, [[Surface Lifts]]). Instructors and classes use the patrol and ski school lane in [[Lift Lines]] once it exists. A lesson runs about two game hours, then the class skis back to the meeting spot. Parents ski on their own meanwhile, and the family plans to meet at the school when the lesson ends. With no instructor free, students wait and get annoyed.
6. **Lessons raise skill.** Each lesson raises the student's skill in the catchment (less each time), and their traits follow. The school popup shows lessons today and students who've moved up a level.
7. **Demand and balance.** Families are drawn by green terrain and a ski school, and their share rises on holidays (Christmas, Presidents' Day). Measure on the Goals Test: share of families, kids' and parents' stars, how often families split, and what a school earns against its wages.

## Open questions

- A kid's day ticket: half the adult price, a separate price, or free under some age?
- Do a family's adults ever split up, one skiing with the kids and one off alone?

## Log

- 2026-10-13: Planned with the user: kids and family groups in the base game, with the Ski School (instructors as staff who lead classes; lessons for kids and beginner adults; skill gains that last).
- 2026-10-13: The gentlest chair is fine for lessons; classes use the ski school lane once there is one (the user). Kids' tickets and adults splitting up left for later.

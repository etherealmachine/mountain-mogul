---
title: Families
kind: plan
status: partial
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

## How it works

- **The school** (`world.ServiceSkiSchool`, not in the demo): a room in the Services menu, $14,000 a tile and $150 a day for the desk, signing up two guests at a time per tile (`PoolLessonDesk`). Its popup sets the lesson price ($90 by default) and the instructors (2 by default, $280 a day each, charged with the building), and shows who's waiting for a class and today's lessons and students who moved up a level. Its meeting spot is a tile out from its door (`Building.MeetingSpot`). Lesson money is its own revenue line.
- **The lesson need** (`ai.NeedLesson`, `ai.OfferLesson`): rolled each visit for 35% of beginners and 5% of intermediates (`LessonShareByTier`). With no school they drop it on arrival, annoyed ("I wanted a lesson, but there's no ski school"). Signing up comes after skis and a ticket and ahead of skiing; the guest pays at the desk, then waits at the meeting spot (`sim/lessons.go`), giving up after 1.5 hours of lift time, or at closing ("no class came", a letdown).
- **Instructors** (`World.Instructors`, not saved; hired to each school's headcount): experts (skill 0.95) in a violet jacket and red skis, waiting by the meeting spot. When eight students wait, or the first has waited ten minutes with the lifts running, an instructor takes up to eight out as a group they lead (`sim/groups.go`): while teaching they're in OnMountain, planning for the class at its weakest skier's level (not its hunger), skiing at 85% of its slowest skier's comfortable speed, and waiting at lift tops for students still in the line below as well as those riding. Their needs stay met, they never go home, and they leave no thoughts, falls or injuries. After two hours (or at closing) they head back to the school with the class; there the lesson's done ("my lesson was great", a highlight), the students rejoin their own groups and plan their day, and the instructor waits for the next class.
- **Learning** (`learn`): a lesson adds 0.06 skill, divided by one more than the lessons the guest has had (`Guest.Lessons`, saved), and their comfortable speed, pitch and aggression follow; tastes and budget stay theirs. The catchment keeps it.

## Steps

In order (the user, 2026-10-13: the Ski School first, for beginner adults, then kids and families). Each step builds with `go build` and `go vet`, and is checked headless on the Boreal Goals Test and on screen.

1. Done: **The Ski School building.** A service room in the [[Building Tool]] with a desk and a meeting spot on the snow outside, best near gentle terrain. The school popup sets the instructor headcount, the wage, and the lesson price. A lesson need is rolled per visit for a share of beginner adults (the Ski School card's lesson need). Guests sign up at the desk and wait at the meeting spot for a class.
2. Done: **Classes on the hill.** An instructor leads a class of up to eight as a group leader, on the existing follower code (`sim/groups.go`), planning at the class's level and keeping to the gentlest lift the school can reach (a chair is fine; a magic carpet where there is one). Instructors and classes use the patrol and ski school lane in [[Lift Lines]] once it exists. A lesson runs about two game hours, then the class skis back to the meeting spot. With no instructor free, students wait and get annoyed.
3. Done: **Lessons raise skill.** Each lesson raises the student's skill in the catchment (less each time), and their traits follow. The school popup shows lessons today and students who've moved up a level.
4. **Kids in the catchment.** An age on each guest (adult or child), saved. `GroupMix` gains a family share, and `formEntryGroups` (`world/group.go`) builds a family as one or two adults and one to three kids. Kids' skill rolls low (mostly beginners, some intermediates). A kid never leads (`joinParty` picks an adult), never drives, and never stays on alone: when the last adult heads home, the kids go too. The family share is in the editor's Guests tab with the rest of the group mix.
5. **Kids look and tire like kids.** A smaller figure (about two-thirds scale, in `place` in `render/figure.go`), with the seat, unload and carry heights adjusted so a kid sits on the chair rather than floating over it. Energy drains faster, so a family needs breaks sooner, and comfort speed sits lower.
6. **Parents' stars follow the kids.** A parent's star level is at most one above their worst-off kid's; a kid's highlight gives the parent one too ("the kids loved it"). Family moments: "waited for the kids at every turn", "the whole family on one chair". Kids' reviews count in the rating like anyone's.
7. **Kids in lessons.** The lesson need is rolled for kids whose parents want harder terrain than they do; parents ski on their own meanwhile, and the family plans to meet at the school when the lesson ends.
8. **Demand and balance.** Families are drawn by green terrain and a ski school, and their share rises on holidays (Christmas, Presidents' Day). Measure on the Goals Test: share of families, kids' and parents' stars, how often families split, what a school earns against its wages.

## Open questions

- A kid's day ticket: half the adult price, a separate price, or free under some age?
- Do a family's adults ever split up, one skiing with the kids and one off alone?

## Log

- 2026-10-13: Planned with the user: kids and family groups in the base game, with the Ski School (instructors as staff who lead classes; lessons for kids and beginner adults; skill gains that last).
- 2026-10-13: The gentlest chair is fine for lessons; classes use the ski school lane once there is one (the user). Kids' tickets and adults splitting up left for later.
- 2026-10-13: Reordered (the user): the Ski School first, for beginner adults, then kids and families, then kids in lessons.
- 2026-10-13: Steps 1–3 built (the school, classes, lasting skill). On the Boreal Goals Test with a two-tile school and two instructors by Lift1: 40–52 sign-ups a day, 29–38 lessons, the rest giving up for want of an instructor; $3,600–4,700 in lessons against $710 a day for the school; 8 of 29 students moved up a level from a first lesson (at a 0.12 gain it was 24 of 32, too fast). While an instructor skis, about half the class is within 60 m and 60% on their line, like friend groups (54%). Without a school, a day's stars are unchanged (2.86★). Took the thought kinds past 64: the conditions mask is two words now.

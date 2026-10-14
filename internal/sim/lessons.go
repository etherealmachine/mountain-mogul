package sim

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/world"
)

// Ski school lessons (notes/next/Families.md): a guest who came wanting a
// lesson (world.Guest.WantsLesson) signs up at a ski school's desk
// (ai.OfferLesson, paying the school's LessonPrice), then waits at the
// school's meeting spot (Building.MeetingSpot) for an instructor to take
// a class out. A guest who waits too long, or is still waiting when the
// lifts close, gives up on it.
//
// Instructors (World.Instructors, one per Building.Instructors at each
// school) are staff who ski: an expert in the school's jacket, waiting at
// the meeting spot. Once ClassSize students wait, or the first has waited
// classGather, an instructor takes up to ClassSize out as a group they
// lead (sim/groups.go): they plan for the class (goap.poolGroup: the
// weakest skier, the most pressing needs), and the students copy their
// plans and follow their line down. While teaching they're in
// OnMountain, so they ski, queue and ride like anyone; their needs stay
// met and they never go home. After lessonLength, or at closing, the
// instructor heads back to the school with the class; there the lesson's
// done (ThoughtGoodLesson), the students go back to their own groups, and
// the instructor waits for the next class.

const (
	// lessonWaitGiveUp is how long a signed-up guest waits for a class
	// before giving up on it, sim seconds.
	lessonWaitGiveUp = world.SimSecondsPerHour * 3 / 2
	// lessonSpacing is the spacing of the students waiting at a meeting
	// spot, metres, lessonRow how many stand in a row.
	lessonSpacing = float32(1.3)
	lessonRow     = 4
	// classGather is how long the first student waits before an
	// instructor takes out whoever's there, sim seconds; lessonLength
	// how long a class is out before heading back, and classReturnMax
	// how long it may take to get back before the lesson ends anyway.
	classGather    = world.SimSecondsPerHour / 6
	lessonLength   = world.SimSecondsPerHour * 2
	classReturnMax = world.SimSecondsPerHour
	// classBackNear is how near the meeting spot, metres, the instructor
	// has to be for the class to be back.
	classBackNear = float32(15)
	// lessonGain is the skill a first lesson adds; each later one adds
	// lessonGain / (1 + lessons had before it).
	lessonGain = float32(0.06)
	// instructorSkill is an instructor's skill: an expert. Teaching,
	// they ski at classPace of the slowest student's comfortable speed.
	instructorSkill = 0.95
	classPace       = 0.85
)

// anySchool reports whether some building signs guests up for lessons.
func anySchool(w *world.World) bool {
	if !world.ServiceSkiSchool.Available() {
		return false
	}
	for _, b := range w.Buildings {
		if b.Type == world.BuildingLodge && b.OffersUse(ai.OfferLesson) {
			return true
		}
	}
	return false
}

// tickLessonWait holds a signed-up guest at their school's meeting spot,
// in their place among the others waiting, facing out from the school;
// they give up after lessonWaitGiveUp, at closing, or if the school's
// gone.
func (s *Simulation) tickLessonWait(a *world.Guest, dt float64) {
	w := s.World
	if a.Lesson.PickUp {
		s.tickPickUp(a, dt)
		return
	}
	if !s.LiftsRunning() && !s.ClosedForDay() {
		a.Lesson.Since = s.SimTime // classes start with the lifts: the wait counts from then
	}
	b := findBuildingByID(w, a.Lesson.SchoolID)
	if b == nil || !b.OffersUse(ai.OfferLesson) || s.ClosedForDay() || s.SimTime-a.Lesson.Since > lessonWaitGiveUp {
		s.giveUpLesson(a)
		return
	}
	spot, out, ok := b.MeetingSpot()
	if !ok {
		s.giveUpLesson(a)
		return
	}
	place := 0
	for _, o := range w.OnMountain {
		if o == a {
			break
		}
		if o.Lesson.SchoolID == b.ID && !o.Lesson.InClass && !o.Lesson.PickUp {
			place++
		}
	}
	side := mgl32.Vec2{-out[1], out[0]}
	col, row := place%lessonRow, place/lessonRow
	at := spot.Add(side.Mul((float32(col) - (lessonRow-1)/2.0) * lessonSpacing)).Add(out.Mul(float32(row) * lessonSpacing))
	target := mgl32.Vec3{at[0], w.Terrain.InterpolatedSurfaceElevationAt(at[0], at[1]), at[1]}
	if d := (mgl32.Vec2{target[0] - a.Pos[0], target[2] - a.Pos[2]}); d.Len() > lineSnap {
		s.shuffleToward(a, target, dt)
		return
	}
	a.Pos, a.Speed = target, 0
	a.Heading = float32(math.Atan2(float64(out[0]), float64(out[1])))
}

// giveUpLesson drops a signed-up guest's lesson: no class came. They plan
// their day from here.
func (s *Simulation) giveUpLesson(a *world.Guest) {
	a.Lesson = world.Lesson{}
	a.WantsLesson = false
	s.applyEvent(a, ai.ThoughtNoClass)
	s.replan(a)
}

// tickSchools keeps each school's instructors, starts classes for the
// students waiting, and runs the classes out (each tick, before the
// guests move).
func (s *Simulation) tickSchools() {
	w := s.World
	if !world.ServiceSkiSchool.Available() {
		return
	}
	s.syncInstructors()
	for _, c := range append([]*world.Class(nil), w.Classes...) {
		s.tickClass(c)
	}
	if !s.LiftsRunning() || s.ClosedForDay() {
		return
	}
	for _, b := range w.Buildings {
		if b.Type != world.BuildingLodge || !b.OffersUse(ai.OfferLesson) {
			continue
		}
		var waiting []*world.Guest
		for _, g := range w.OnMountain {
			if g.Lesson.SchoolID == b.ID && !g.Lesson.InClass && !g.Lesson.PickUp && g.RestTimer <= 0 && !g.Fallen && !g.Removed {
				waiting = append(waiting, g)
			}
		}
		for len(waiting) > 0 {
			first := waiting[0]
			for _, g := range waiting {
				if g.Lesson.Since < first.Lesson.Since {
					first = g
				}
			}
			if len(waiting) < world.ClassSize && s.SimTime-first.Lesson.Since < classGather {
				break
			}
			in := s.idleInstructor(b.ID)
			if in == nil {
				break
			}
			n := min(len(waiting), world.ClassSize)
			s.startClass(b, in, waiting[:n])
			waiting = waiting[n:]
		}
	}
}

// syncInstructors keeps World.Instructors to each school's headcount:
// new ones at the meeting spot, and spare ones (idle first) let go.
func (s *Simulation) syncInstructors() {
	w := s.World
	want := map[uint64]int{}
	for _, b := range w.Buildings {
		if b.Type == world.BuildingLodge && b.OffersUse(ai.OfferLesson) {
			want[b.ID] = b.Instructors
		}
	}
	have := map[uint64]int{}
	kept := w.Instructors[:0]
	for _, in := range w.Instructors {
		if have[in.InstructorOf] >= want[in.InstructorOf] && in.Class == nil {
			continue // let go
		}
		have[in.InstructorOf]++
		kept = append(kept, in)
	}
	w.Instructors = kept
	for id, n := range want {
		b := findBuildingByID(w, id)
		for have[id] < n && b != nil {
			s.hireInstructor(b)
			have[id]++
		}
	}
	// Teaching past the headcount (it was cut): the class ends now.
	for _, c := range w.Classes {
		if have[c.SchoolID] > want[c.SchoolID] && !c.Ending {
			s.endClass(c)
			have[c.SchoolID]--
		}
	}
}

// hireInstructor adds an instructor at school b's meeting spot.
func (s *Simulation) hireInstructor(b *world.Building) {
	w := s.World
	spot, out, _ := b.MeetingSpot()
	g := &world.Guest{
		ID:            w.NextID(),
		State:         world.OnMountain,
		Discipline:    world.Ski,
		Traits:        ai.TraitsFor(instructorSkill),
		Balance:       1,
		HasSeasonPass: true,
		SkisOn:        true,
		InstructorOf:  b.ID,
	}
	g.RemainingBudget = 1e6 // the school's account, for lunch with the class
	topUpInstructor(g)
	s.standAtSpot(g, b, spot, out, len(w.Instructors))
	w.Instructors = append(w.Instructors, g)
}

// topUpInstructor keeps an instructor fed, rested and patient: staff
// on the job.
func topUpInstructor(g *world.Guest) {
	g.Hunger, g.Thirst, g.Energy, g.Patience = 1, 1, 1, 1
	g.Chill = 0
}

// standAtSpot puts an idle instructor beside school b's meeting spot,
// facing out from the school.
func (s *Simulation) standAtSpot(g *world.Guest, b *world.Building, spot, out mgl32.Vec2, i int) {
	side := mgl32.Vec2{-out[1], out[0]}
	at := spot.Add(side.Mul((lessonRow/2 + 1 + float32(i%3)) * lessonSpacing)).Sub(out.Mul(float32(i/3) * lessonSpacing))
	g.Pos = mgl32.Vec3{at[0], s.World.Terrain.InterpolatedSurfaceElevationAt(at[0], at[1]), at[1]}
	g.Heading = float32(math.Atan2(float64(out[0]), float64(out[1])))
	g.Speed = 0
}

// idleInstructor is one of school id's instructors not teaching, or nil.
func (s *Simulation) idleInstructor(id uint64) *world.Guest {
	for _, in := range s.World.Instructors {
		if in.InstructorOf == id && in.Class == nil {
			return in
		}
	}
	return nil
}

// startClass sends instructor in out from school b with students, who
// leave their own groups to follow them.
func (s *Simulation) startClass(b *world.Building, in *world.Guest, students []*world.Guest) {
	w := s.World
	c := &world.Class{SchoolID: b.ID, Instructor: in, Start: s.SimTime}
	in.Class = c
	in.Party = world.Party{}
	in.Plan = ai.Plan{}
	in.Route = world.SkiRoute{}
	in.Removed = false
	topUpInstructor(in)
	// Ahead of the students in OnMountain, as a group's leader is: they
	// plan, and start their line, before the class follows it.
	at := len(w.OnMountain)
	for i, g := range w.OnMountain {
		for _, st := range students {
			if g == st && i < at {
				at = i
			}
		}
	}
	w.OnMountain = append(w.OnMountain, nil)
	copy(w.OnMountain[at+1:], w.OnMountain[at:])
	w.OnMountain[at] = in
	for i, g := range students {
		leaveParty(g)
		g.Party = world.Party{Leader: in, Rank: i + 1}
		side := float32(followSideMin) + followSideStep*float32(i/2)
		if i%2 == 1 {
			side = -side
		}
		g.Party.FollowSide = min(side, followSideMax)
		in.Party.Followers = append(in.Party.Followers, g)
		g.Lesson.InClass = true
		g.Class = c
		g.Plan, g.Path, g.PathIdx, g.TargetID = ai.Plan{}, nil, 0, 0
		c.Students = append(c.Students, g)
	}
	// They ski at the class's pace: a little under its slowest skier's.
	for _, g := range students {
		in.Traits.ComfortSpeed = min(in.Traits.ComfortSpeed, g.Traits.ComfortSpeed*classPace)
	}
	w.Classes = append(w.Classes, c)
	s.replan(in)
}

// tickClass runs class c: the instructor kept topped up, students who've
// left dropped, and, once its time is up, back to the school and done.
func (s *Simulation) tickClass(c *world.Class) {
	w := s.World
	in := c.Instructor
	topUpInstructor(in)
	kept := c.Students[:0]
	for _, g := range c.Students {
		if g.Removed || g.State != world.OnMountain {
			g.Class = nil
			continue
		}
		kept = append(kept, g)
	}
	c.Students = kept
	b := findBuildingByID(w, c.SchoolID)
	if b == nil || !b.OffersUse(ai.OfferLesson) || len(c.Students) == 0 {
		s.finishClass(c, len(c.Students) > 0)
		return
	}
	if !c.Ending && (s.SimTime-c.Start >= lessonLength || s.ClosedForDay()) {
		s.endClass(c)
	}
	if !c.Ending {
		return
	}
	spot, _, _ := b.MeetingSpot()
	if (mgl32.Vec2{in.Pos[0], in.Pos[2]}).Sub(spot).Len() <= classBackNear || s.SimTime-c.EndSince >= classReturnMax {
		s.finishClass(c, true)
	}
}

// endClass sends class c back to its school: the instructor's plan, which
// the class takes up.
func (s *Simulation) endClass(c *world.Class) {
	c.Ending, c.EndSince = true, s.SimTime
	in := c.Instructor
	if in.OnLiftID != 0 || in.Queued || in.Unload.LiftID != 0 {
		return // on the way up: they head back once off (tickClass replans)
	}
	s.headBack(c)
}

// backToSchool is the goal of an instructor's plan back to the school.
const backToSchool = "BackToSchool"

// headBack gives the instructor of class c a plan back to its school,
// shared with the class.
func (s *Simulation) headBack(c *world.Class) {
	in := c.Instructor
	done := min(in.Plan.Step, len(in.Plan.Steps))
	in.Plan = ai.Plan{GoalName: backToSchool, Steps: []ai.PlanAction{{Kind: ai.ActSkiToService, BldgID: c.SchoolID}}}
	in.Route = world.SkiRoute{}
	s.share(in, s.planFrom(in), in.Plan, done)
	s.onPlanStepStart(in)
}

// finishClass ends class c: with done, each student has had their lesson;
// they go back to their own groups and plan their day, and the
// instructor waits at the school for the next class.
func (s *Simulation) finishClass(c *world.Class, done bool) {
	w := s.World
	for _, g := range c.Students {
		g.Class = nil
		g.Lesson = world.Lesson{}
		if done {
			g.WantsLesson = false
			s.applyEvent(g, ai.ThoughtGoodLesson)
			s.learn(g, findBuildingByID(w, c.SchoolID))
		}
		g.Party = world.Party{}
		g.Plan = ai.Plan{}
		if g.Child && adultOut(w, g) && atSchool(w, g, c.SchoolID) {
			// A kid back at the school waits for a parent to collect
			// them, out of the family's group till then (tickPickUp);
			// one who lost the class carries on with their family.
			g.Lesson = world.Lesson{SchoolID: c.SchoolID, Since: s.SimTime, PickUp: true}
			continue
		}
		s.joinParty(g)
	}
	in := c.Instructor
	in.Class = nil
	in.Party = world.Party{}
	in.Traits.ComfortSpeed = ai.TraitsFor(instructorSkill).ComfortSpeed
	in.Plan, in.Path, in.PathIdx, in.TargetID = ai.Plan{}, nil, 0, 0
	in.Queued, in.OnLiftID, in.Unload = false, 0, world.Unloading{}
	in.Fallen, in.Tumble = false, world.Tumble{}
	for _, l := range w.Lifts {
		l.LeaveLine(in)
		for i := range l.Chairs {
			for j, q := range l.Chairs[i].Passengers {
				if q == in {
					l.Chairs[i].Passengers[j] = nil
				}
			}
		}
	}
	for i, g := range w.OnMountain {
		if g == in {
			w.OnMountain = append(w.OnMountain[:i], w.OnMountain[i+1:]...)
			break
		}
	}
	if b := findBuildingByID(w, c.SchoolID); b != nil {
		if spot, out, ok := b.MeetingSpot(); ok {
			n := 0
			for _, o := range w.Instructors {
				if o == in {
					break
				}
				if o.InstructorOf == c.SchoolID {
					n++
				}
			}
			s.standAtSpot(in, b, spot, out, n)
		}
	}
	for i, x := range w.Classes {
		if x == c {
			w.Classes = append(w.Classes[:i], w.Classes[i+1:]...)
			break
		}
	}
}

// learn raises g's skill for a lesson at school b, for good (the
// catchment keeps it): less for each lesson they've had, and their
// comfortable speed and pitch follow. Their tastes and budget stay their
// own.
func (s *Simulation) learn(g *world.Guest, b *world.Building) {
	before := g.Traits.Skill
	skill := min(before+lessonGain/float32(1+g.Lessons), 1)
	g.Lessons++
	t := world.TraitsFor(skill, g.Child)
	g.Traits.Skill = skill
	g.Traits.ComfortSpeed, g.Traits.ComfortSlope, g.Traits.Aggression = t.ComfortSpeed, t.ComfortSlope, t.Aggression
	if b == nil {
		return
	}
	if day := int(s.SimTime / world.SecondsPerSimDay); b.LessonDay != day {
		b.LessonDay, b.LessonsToday, b.MovedUpToday = day, 0, 0
	}
	b.LessonsToday++
	if world.SkillTier(skill) > world.SkillTier(before) {
		b.MovedUpToday++
	}
}

// pickUpNear is how near a kid waiting at the school a parent must come,
// metres, to collect them.
const pickUpNear = float32(30)

// adultOut reports whether kid g has an adult of their family out.
func adultOut(w *world.World, g *world.Guest) bool {
	for _, m := range w.GroupOf(g) {
		if !m.Child && m.State == world.OnMountain && !m.Removed {
			return true
		}
	}
	return false
}

// tickPickUp holds a kid back from a lesson at the meeting spot until an
// adult of their family comes for them (collectKids sends one), then
// back with the family they go.
func (s *Simulation) tickPickUp(k *world.Guest, dt float64) {
	w := s.World
	b := findBuildingByID(w, k.Lesson.SchoolID)
	spot, out, ok := mgl32.Vec2{}, mgl32.Vec2{}, false
	if b != nil {
		spot, out, ok = b.MeetingSpot()
	}
	for _, m := range w.GroupOf(k) {
		if !m.Child && m.State == world.OnMountain && !m.Removed && m.Pos.Sub(k.Pos).Len() <= pickUpNear || !ok {
			k.Lesson = world.Lesson{}
			k.Party = world.Party{}
			s.joinParty(k)
			k.Plan = ai.Plan{}
			return
		}
	}
	side := mgl32.Vec2{-out[1], out[0]}
	at := spot.Sub(side.Mul((lessonRow/2 + 1) * lessonSpacing))
	target := mgl32.Vec3{at[0], w.Terrain.InterpolatedSurfaceElevationAt(at[0], at[1]), at[1]}
	if d := (mgl32.Vec2{target[0] - k.Pos[0], target[2] - k.Pos[2]}); d.Len() > 2 {
		s.shuffleToward(k, target, dt)
		return
	}
	k.Speed = 0
}

// collectKids sends adult a, free to go, to the ski school where one of
// their kids is waiting to be collected (when no other adult of theirs
// already is): a plan to ski there, shared with whoever's with them.
func (s *Simulation) collectKids(a *world.Guest) {
	if a.Child || a.GroupID == 0 || a.Party.Leader != nil || a.InstructorOf != 0 || goingHome(a) ||
		a.OnLiftID != 0 || a.Queued || a.Unload.LiftID != 0 || a.Fallen || a.RestTimer > 0 || a.Visit.Waiting {
		return
	}
	var school uint64
	for _, m := range s.World.GroupOf(a) {
		if m.Child && m.Lesson.PickUp && m.State == world.OnMountain && !m.Removed {
			school = m.Lesson.SchoolID
		}
		if !m.Child && m != a && m.Plan.GoalName == collectKidsGoal {
			return // someone's on their way
		}
	}
	if school == 0 || a.Plan.GoalName == collectKidsGoal {
		return
	}
	done := min(a.Plan.Step, len(a.Plan.Steps))
	a.Plan = ai.Plan{GoalName: collectKidsGoal, Steps: []ai.PlanAction{{Kind: ai.ActSkiToService, BldgID: school}}}
	a.Route = world.SkiRoute{}
	s.share(a, s.planFrom(a), a.Plan, done)
	s.onPlanStepStart(a)
}

// collectKidsGoal is the goal of a parent's plan to collect a kid from
// the ski school.
const collectKidsGoal = "CollectKids"

// atSchool reports whether g is back by school id's meeting spot.
func atSchool(w *world.World, g *world.Guest, id uint64) bool {
	b := findBuildingByID(w, id)
	if b == nil || g.OnLiftID != 0 || g.Queued || g.Unload.LiftID != 0 {
		return false
	}
	spot, _, ok := b.MeetingSpot()
	return ok && (mgl32.Vec2{g.Pos[0], g.Pos[2]}).Sub(spot).Len() <= 2*classBackNear
}

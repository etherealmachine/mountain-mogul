package world

import (
	"math"
	"math/rand"

	"github.com/go-gl/mathgl/mgl32"
	"mountain-mogul/internal/ai"
)

// Groups: guests who come together ski together (notes/next/Groups.md).
// A group is formed in the catchment before anyone visits: its members
// share a GroupID (the first member's ID) and the demand poll rolls their
// visit once. On the mountain one member leads: plans, routes round the
// trees, and steers with the full fan; the others copy the leader's plans
// and ski the leader's line with their own bodies (sim/groups.go).

// MaxGroupSize is the largest group: a ski lesson, an instructor and a
// class of twelve.
const MaxGroupSize = 13

// GroupMix is how a scenario's guests come: the mean size of a group
// (lessons aside), the share of groups that are a lesson of
// MaxGroupSize beginners, and the share whose members' skills are
// rolled separately (the rest share the first member's skill tier).
type GroupMix struct {
	MeanSize float32
	Lessons  float32
	Mixed    float32
}

// DefaultGroupMix is the mix a scenario gets unless it says otherwise:
// groups of about two and a half, as carloads were before groups, about
// half of them mixed in skill.
var DefaultGroupMix = GroupMix{MeanSize: 2.4, Lessons: 0.01, Mixed: 0.5}

// Mix is the world's group mix, DefaultGroupMix when it hasn't one.
func (w *World) Mix() GroupMix {
	if w.GroupMix == (GroupMix{}) {
		return DefaultGroupMix
	}
	return w.GroupMix
}

// rollGroupSize draws a group's size outside lessons: one plus a
// Poisson draw for the members beyond the first, so the mean is
// mix.MeanSize, kept below a lesson's size.
func rollGroupSize(mix GroupMix, r *rand.Rand) int {
	lambda := float64(max(mix.MeanSize, 1) - 1)
	l, k, p := math.Exp(-lambda), 0, 1.0
	for {
		p *= r.Float64()
		if p <= l {
			break
		}
		k++
	}
	return min(1+k, MaxGroupSize-1)
}

// FormGroups puts every guest without a group into one: guests from the
// same road entry, in groups rolled from the world's mix. A same-skill
// group takes its members from the first member's tier while it has
// guests left; a lesson takes beginners. Bumps GroupsRev.
func FormGroups(w *World, seed int64) {
	mix := w.Mix()
	byEntry := map[uint64][]*Guest{}
	var entries []uint64
	for _, g := range w.Guests {
		if g.GroupID != 0 {
			continue
		}
		if _, ok := byEntry[g.HomeEntryID]; !ok {
			entries = append(entries, g.HomeEntryID)
		}
		byEntry[g.HomeEntryID] = append(byEntry[g.HomeEntryID], g)
	}
	for _, e := range entries {
		r := rand.New(rand.NewSource(seed ^ int64(e)*104729 ^ 0x6a0))
		formEntryGroups(byEntry[e], mix, r)
	}
	w.GroupsRev++
}

// formEntryGroups groups guests, all from one entry.
func formEntryGroups(guests []*Guest, mix GroupMix, r *rand.Rand) {
	r.Shuffle(len(guests), func(i, j int) { guests[i], guests[j] = guests[j], guests[i] })
	var tiers [3][]*Guest
	for _, g := range guests {
		t := SkillTier(g.Traits.Skill)
		tiers[t] = append(tiers[t], g)
	}
	used := map[*Guest]bool{}
	// take pops the next unused guest from list, nil when it's empty.
	take := func(list *[]*Guest) *Guest {
		for len(*list) > 0 {
			g := (*list)[len(*list)-1]
			*list = (*list)[:len(*list)-1]
			if !used[g] {
				used[g] = true
				return g
			}
		}
		return nil
	}
	all := append([]*Guest(nil), guests...)
	for {
		size, from := rollGroupSize(mix, r), -1 // from: the tier members come from; -1 any
		lesson := r.Float32() < mix.Lessons
		var first *Guest
		if lesson {
			size, from = MaxGroupSize, 0
			first = take(&tiers[0])
		} else {
			first = take(&all)
		}
		if first == nil {
			return
		}
		if !lesson && r.Float32() >= mix.Mixed {
			from = SkillTier(first.Traits.Skill)
		}
		first.GroupID = first.ID
		for n := 1; n < size; n++ {
			var m *Guest
			if from >= 0 {
				m = take(&tiers[from])
			}
			if m == nil {
				m = take(&all)
			}
			if m == nil {
				return
			}
			m.GroupID = first.ID
		}
	}
}

// SkillTier is a skill's tier: 0 beginner, 1 intermediate, 2 advanced.
func SkillTier(skill float32) int {
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		return 2
	case skill >= ai.SkillIntermediateThreshold:
		return 1
	}
	return 0
}

// RegroupGuests breaks up every group whose members are all at home and
// forms groups again from the world's mix, as after the mix is edited.
func RegroupGuests(w *World, seed int64) {
	out := map[uint64]bool{} // groups with someone out
	for _, g := range w.Guests {
		if g.State != AtHome && g.GroupID != 0 {
			out[g.GroupID] = true
		}
	}
	for _, g := range w.Guests {
		if !out[g.GroupID] {
			g.GroupID = 0
		}
	}
	FormGroups(w, seed)
}

// GuestGroups lists the catchment's groups, each in pool order, with
// guests outside any group as groups of one. Cached until GroupsRev or
// the pool changes.
func (w *World) GuestGroups() [][]*Guest {
	if w.groups.rev == w.GroupsRev && w.groups.n == len(w.Guests) && w.groups.list != nil {
		return w.groups.list
	}
	idx := map[uint64]int{}
	var list [][]*Guest
	for _, g := range w.Guests {
		if g.GroupID == 0 {
			list = append(list, []*Guest{g})
			continue
		}
		i, ok := idx[g.GroupID]
		if !ok {
			i = len(list)
			idx[g.GroupID] = i
			list = append(list, nil)
		}
		list[i] = append(list[i], g)
	}
	w.groups.list, w.groups.rev, w.groups.n = list, w.GroupsRev, len(w.Guests)
	w.groups.byID = idx
	return list
}

// GroupOf is g's group (g alone when it has none).
func (w *World) GroupOf(g *Guest) []*Guest {
	if g.GroupID == 0 {
		return []*Guest{g}
	}
	w.GuestGroups()
	if i, ok := w.groups.byID[g.GroupID]; ok {
		return w.groups.list[i]
	}
	return []*Guest{g}
}

// groupCache is World.GuestGroups' cache.
type groupCache struct {
	list [][]*Guest
	byID map[uint64]int
	rev  int
	n    int
}

// Party is a guest's group on the mountain today (sim/groups.go). Not
// saved: a loaded game regroups the guests who are out.
type Party struct {
	// Leader is who the guest follows; nil when they lead (or ski
	// alone). Followers are the leader's.
	Leader    *Guest
	Followers []*Guest
	// Shared is a ring of the leader's latest plans, for followers to
	// copy; SharedHead is the next to write.
	Shared     [SharedPlans]SharedPlan
	SharedHead int
	// Gen is the leader's plan the guest is on (SharedPlan.Gen), for a
	// leader or a follower skiing a copy of it; 0 for a follower's own.
	Gen uint32
	// PlanAt is when the guest's plan was last set; a follower copies
	// only plans the leader made since. WaitSince is when a follower
	// started waiting for the leader, 0 when not waiting.
	PlanAt    float64
	WaitSince float64
	// TopLift is the lift a follower just got off, waiting at its top
	// for a leader still on it (since TopSince); 0 when not waiting.
	TopLift  uint64
	TopSince float64
	// Crumbs is the leader's line, while they lead anyone.
	Crumbs *Crumbs
	// A follower's place on the line: the crumb they're at (of Seq),
	// their side of it in metres, and their place in the group.
	FollowSeq  uint32
	FollowIdx  int
	FollowSide float32
	Rank       int
}

// WithLeader reports whether the follower f is skiing one of their
// leader's recent plans (not lost, or planning for themselves).
func (f *Guest) WithLeader() bool {
	l := f.Party.Leader
	if l == nil || f.Party.Gen == 0 {
		return false
	}
	for _, sp := range l.Party.Shared {
		if sp.Gen == f.Party.Gen {
			return true
		}
	}
	return false
}

// SharedPlans is how many of the leader's plans a follower can pick from.
const SharedPlans = 8

// SharedPlan is a plan the leader made: its number (Gen), the plan it
// replaced (Prev, 0 for none) after how many of that plan's steps
// (Done), where it was made from, and when.
type SharedPlan struct {
	Gen, Prev uint32
	Done      int
	From      PlanFrom
	At        float64
	Plan      ai.Plan
}

// PlanFrom is where a plan was made from: the top of a lift, or arriving
// at a lot; zero for anywhere else.
type PlanFrom struct {
	LiftTop, Arrive uint64
}

// CrumbRing is how many crumbs a leader's line holds.
const CrumbRing = 256

// Crumbs is a leader's line on one descent: a crumb every few metres,
// with the distance along it, in a ring. Seq numbers the descents; End
// is where the descent goes (a lift or building ID); N counts the
// crumbs dropped on it, so crumb i is Pts[i%CrumbRing] while i ≥
// N-CrumbRing.
type Crumbs struct {
	Seq uint32
	// End is where the leader's descent goes, and PrevEnd where it went
	// before their plan changed on the way (followers still on the
	// old plan keep to the line).
	End, PrevEnd uint64
	N            int
	Last         float64 // SimTime of the last crumb
	Pts          [CrumbRing]Crumb
}

// Crumb is one point on a leader's line, S metres from its start.
type Crumb struct {
	P mgl32.Vec2
	S float32
}

// At is crumb i.
func (c *Crumbs) At(i int) Crumb { return c.Pts[i%CrumbRing] }

// Oldest is the oldest crumb still held.
func (c *Crumbs) Oldest() int { return max(0, c.N-CrumbRing) }

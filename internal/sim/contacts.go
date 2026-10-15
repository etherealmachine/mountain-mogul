package sim

import (
	"fmt"
	"io"
	"math"
	"sort"

	"mountain-mogul/internal/ai"
	"mountain-mogul/internal/rng"
	"mountain-mogul/internal/world"

	"github.com/go-gl/mathgl/mgl32"
)

// Skier contacts and collisions (notes/next/Skier Collisions.md). A
// contact is a skier passing within contactRadius of someone on the
// snow during a step: another skier, someone fallen, or someone standing
// with their skis on. Walkers, lift lines, riders and guests getting off
// a lift don't count. Steps are up to 0.2 s, so the check is on the
// closest approach along both moves, not where they end up; a pair still
// touching from the step before isn't counted again.
//
// What a contact does depends on the closing speed (along the line
// between them) against each body's collideAt, which grows with skill:
// past it they go down, short of it they lose some balance (a brush).
// Group members skiing together don't collide, and a slow brush near a
// lift's base or top (people milling about) costs nothing.
const (
	contactRadius = float32(0.8) // two bodies with skis
	// contactMaxSpeed bounds how far anyone else can have moved in the
	// step, for the grid search (bucketed where they started it).
	contactMaxSpeed = float32(30)

	// collideBase + collideSkill × skill is the closing speed, m/s, that
	// knocks a skier down: 3 for a novice, 8 for an expert.
	collideBase  = float32(3)
	collideSkill = float32(5)
	// A brush costs up to brushBalance (just short of collideAt), from
	// brushMin m/s.
	brushBalance = float32(0.5)
	brushMin     = float32(1)
	// closeCallSpeed is the slowest brush the one brushed past minds.
	closeCallSpeed = float32(2)
	// Injury: a chance growing with closing speed to collideInjuryMax at
	// injuryMaxSpeed, collideHitScale times that for the one hit (they
	// didn't see it coming), of which collideSeriousShare are serious.
	collideInjuryMax    = float32(0.3)
	collideHitScale     = float32(1.5)
	collideSeriousShare = float32(0.4)
	// collideThrow is the share of the closing speed the one hit slides
	// away with, up to collideThrowMax.
	collideThrow    = float32(0.5)
	collideThrowMax = float32(4)
)

// Contact kinds: who the skier came into.
const (
	ContactSkier = iota
	ContactFallen
	ContactStanding
	contactKinds
)

// Closing-speed bands, m/s: a brush (under 2), then 2–5, 5–10, 10 and up.
var contactBands = [...]float32{2, 5, 10}

// Headings between two skiers in contact: the same way (within 45°, one
// overtaking the other), crossing, or head on (over 135° apart).
const (
	ContactSameWay = iota
	ContactCrossing
	ContactHeadOn
	contactAngles
)

// contactStation is how near a lift's base or top a contact counts as
// at the station (loading, unloading, gathering), m.
const contactStation = 40

// ContactStats tallies skier contacts since the last TakeContactStats.
// Contacts within a group (same party or class) count only in
// SameGroup, and those near a lift station only in AtStation (with
// StationKind); the rest are out on the snow, in the other fields.
// Collisions (someone went down), Knocked and Injured count everywhere.
type ContactStats struct {
	N           int
	SameGroup   int
	AtStation   int
	StationKind [contactKinds]int
	Kind        [contactKinds]int
	Band        [len(contactBands) + 1]int
	Angle       [contactAngles]int // skier-on-skier only
	Standing    map[string]int     // what the one standing was doing
	ByTrail     map[uint64]int     // the skier's trail; 0 off any run
	Spots       map[[2]int]int     // by contactSpot squares, metres / contactSpot
	MaxClose    float32
	Collisions  int
	Knocked     int
	Injured     int
}

// contactSpot is the side of the squares contacts are mapped in, m.
const contactSpot = 20

// contactPair is a pair's guest IDs, lower first.
type contactPair [2]uint64

func pairOf(a, b *world.Guest) contactPair {
	if a.ID < b.ID {
		return contactPair{a.ID, b.ID}
	}
	return contactPair{b.ID, a.ID}
}

// skierMove is a skier and where they started this step's move.
type skierMove struct {
	a    *world.Guest
	from mgl32.Vec3
}

// noteSkierFrom records where a skier starts this step's move, before
// applySkier.
func (s *Simulation) noteSkierFrom(a *world.Guest) {
	if s.skierFrom == nil {
		s.skierFrom = map[*world.Guest]mgl32.Vec3{}
	}
	s.skierFrom[a] = a.Pos
	s.skierMoves = append(s.skierMoves, skierMove{a, a.Pos})
}

// contact is one new contact of skier a with b.
type contact struct {
	a, b      *world.Guest
	kind      int
	n         mgl32.Vec2 // unit, a toward b at the closest point
	closing   float32    // m/s
	aHit      bool       // a ran into b, rather than b into a
	sameGroup bool
	atStation bool
}

// stepContacts finds this step's new contacts among the skiers who
// moved, in the order they moved, and plays each out.
func (s *Simulation) stepContacts(dt float64) {
	if s.contacts.ByTrail == nil {
		s.contacts.ByTrail, s.contacts.Spots, s.contacts.Standing = map[uint64]int{}, map[[2]int]int{}, map[string]int{}
	}
	if s.touching == nil {
		s.touching, s.wasTouching = map[contactPair]bool{}, map[contactPair]bool{}
	}
	s.wasTouching, s.touching = s.touching, s.wasTouching
	clear(s.touching)
	const r2 = contactRadius * contactRadius
	for _, m := range s.skierMoves {
		a, from := m.a, m.from
		if a.Removed || a.Player {
			continue
		}
		reach := contactRadius + 1 + (a.Speed+contactMaxSpeed)*float32(dt)
		s.spatial.forEachWithin(from[0], from[2], reach, func(b *world.Guest) {
			if b == a || b.Removed || b.Player {
				return
			}
			bFrom, bSkied := s.skierFrom[b]
			if bSkied && b.ID < a.ID {
				return // found from b's side
			}
			kind := ContactSkier
			switch {
			case bSkied:
			case b.Fallen:
				kind, bFrom = ContactFallen, b.Pos
			case b.Queued || b.OnLiftID != 0 || b.Unload.LiftID != 0 || !b.SkisOn:
				return
			default:
				kind, bFrom = ContactStanding, b.Pos
			}
			// Closest approach over the step, both moving straight.
			p0 := mgl32.Vec2{bFrom[0] - from[0], bFrom[2] - from[2]}
			p1 := mgl32.Vec2{b.Pos[0] - a.Pos[0], b.Pos[2] - a.Pos[2]}
			d := p1.Sub(p0)
			t := float32(0)
			if dd := d.Dot(d); dd > 1e-9 {
				t = clamp32(-p0.Dot(d)/dd, 0, 1)
			}
			rel := p0.Add(d.Mul(t))
			if rel.Dot(rel) > r2 {
				return
			}
			pair := pairOf(a, b)
			s.touching[pair] = true
			if s.wasTouching[pair] {
				return
			}
			c := s.measureContact(a, b, kind, rel)
			s.tallyContact(c)
			s.collide(c)
		})
	}
	clear(s.skierFrom)
	s.skierMoves = s.skierMoves[:0]
}

// measureContact works out a contact of skier a with b, rel apart (b's
// position less a's) at the closest point: how fast they closed, who
// ran into whom, and whether it's within a group or at a station.
func (s *Simulation) measureContact(a, b *world.Guest, kind int, rel mgl32.Vec2) contact {
	c := contact{a: a, b: b, kind: kind}
	va := headingVec(a.Heading).Mul(a.Speed)
	var vb mgl32.Vec2
	if kind == ContactSkier {
		vb = headingVec(b.Heading).Mul(b.Speed)
	}
	// Closing speed along the line between them; the whole relative
	// speed when they're on top of each other.
	dv := va.Sub(vb)
	if l := rel.Len(); l > 0.05 {
		c.n = rel.Mul(1 / l)
		c.closing = max(dv.Dot(c.n), 0)
	} else if l := dv.Len(); l > 0.01 {
		c.n = dv.Mul(1 / l)
		c.closing = l
	} else {
		c.n = headingVec(a.Heading)
	}
	// Whoever was moving more toward the other ran into them.
	c.aHit = kind != ContactSkier || va.Dot(c.n) >= -vb.Dot(c.n)
	c.sameGroup = partyLeader(a) == partyLeader(b) || a.Class != nil && a.Class == b.Class
	p := mgl32.Vec2{a.Pos[0], a.Pos[2]}
	for _, l := range s.World.Lifts {
		if p.Sub(l.Base).Len() < contactStation || p.Sub(l.Top).Len() < contactStation {
			c.atStation = true
			break
		}
	}
	return c
}

// collideAt is the closing speed that knocks g down.
func collideAt(g *world.Guest) float32 {
	return collideBase + collideSkill*clamp32(g.Traits.Skill, 0, 1)
}

// collide plays a contact out: whoever it was too hard for goes down,
// perhaps hurt, and the rest lose some balance.
func (s *Simulation) collide(c contact) {
	if c.sameGroup {
		return // skiing together: they keep out of each other's way
	}
	hitter, hit := c.a, c.b
	if !c.aHit {
		hitter, hit = c.b, c.a
	}
	var down [2]bool
	for i, g := range [2]*world.Guest{hitter, hit} {
		if g.Fallen {
			continue
		}
		at := collideAt(g)
		switch {
		case c.closing >= at:
			down[i] = true
		case c.closing >= brushMin && !c.atStation:
			g.Balance = max(g.Balance-brushBalance*c.closing/at, 0.01)
		}
	}
	if !down[0] && !down[1] {
		if c.closing >= closeCallSpeed && !c.atStation && !hit.Fallen {
			s.applyEvent(hit, ai.ThoughtCloseCall)
		}
		return
	}
	s.contacts.Collisions++
	// Away from each other: the hitter over their tips, the one hit
	// thrown along the impact.
	push := c.n
	if !c.aHit {
		push = push.Mul(-1)
	}
	if down[0] {
		s.knockDown(hitter, world.FallForward, world.FallCollision)
		s.collisionInjury(hitter, c.closing, 1)
	}
	if down[1] {
		s.knockDown(hit, world.FallSide, world.FallCollision)
		v := push.Mul(min(c.closing*collideThrow, collideThrowMax))
		hit.Tumble.Vel = [2]float32{v[0], v[1]}
		s.collisionInjury(hit, c.closing, collideHitScale)
	}
	if !hit.Fallen || down[1] {
		s.applyEvent(hit, ai.ThoughtCrashedInto)
	}
	s.applyEvent(hitter, ai.ThoughtRanInto)
}

// collisionInjury rolls for an injury to g, just knocked down by a
// collision closing at closing m/s, scale times the usual chance.
func (s *Simulation) collisionInjury(g *world.Guest, closing, scale float32) {
	s.contacts.Knocked++
	chance := clamp32(closing/injuryMaxSpeed, 0, 1) * collideInjuryMax * scale
	if rng.Global().Float32() < chance {
		s.contacts.Injured++
		s.injure(g, rng.Global().Float32() < collideSeriousShare)
	}
}

// tallyContact counts one new contact.
func (s *Simulation) tallyContact(c contact) {
	t := &s.contacts
	t.N++
	if c.sameGroup {
		t.SameGroup++
		return
	}
	if c.atStation {
		t.AtStation++
		t.StationKind[c.kind]++
		return
	}
	a, b := c.a, c.b
	t.Kind[c.kind]++
	if c.kind == ContactStanding {
		t.Standing[world.Activity(s.World, b)]++
	}
	if c.kind == ContactSkier {
		cos := headingVec(a.Heading).Dot(headingVec(b.Heading))
		switch {
		case cos > float32(math.Cos(math.Pi/4)):
			t.Angle[ContactSameWay]++
		case cos < -float32(math.Cos(math.Pi/4)):
			t.Angle[ContactHeadOn]++
		default:
			t.Angle[ContactCrossing]++
		}
	}
	band := len(contactBands)
	for i, v := range contactBands {
		if c.closing < v {
			band = i
			break
		}
	}
	t.Band[band]++
	t.MaxClose = max(t.MaxClose, c.closing)
	id := plannedTrail(a)
	if id == 0 {
		if tr := s.World.TrailAt(int(a.Pos[0]/CellSize), int(a.Pos[2]/CellSize)); tr != nil {
			id = tr.ID
		}
	}
	t.ByTrail[id]++
	t.Spots[[2]int{int(a.Pos[0] / contactSpot), int(a.Pos[2] / contactSpot)}]++
}

func headingVec(h float32) mgl32.Vec2 {
	return mgl32.Vec2{float32(math.Sin(float64(h))), float32(math.Cos(float64(h)))}
}

// partyLeader is who leads a's group: a when they lead or ski alone.
func partyLeader(a *world.Guest) *world.Guest {
	if a.Party.Leader != nil {
		return a.Party.Leader
	}
	return a
}

// TakeContactStats returns the contacts counted since the last call and
// starts counting afresh.
func (s *Simulation) TakeContactStats() ContactStats {
	c := s.contacts
	s.contacts = ContactStats{}
	return c
}

// Write prints the tally: collisions, totals, then the contacts out on
// the snow by kind, closing speed and heading, the runs and spots with
// the most, and what standing guests were doing.
func (c ContactStats) Write(out io.Writer, w *world.World) {
	fmt.Fprintf(out, "collisions %d: %d knocked down, %d hurt\n", c.Collisions, c.Knocked, c.Injured)
	fmt.Fprintf(out, "  contacts %d: same group %d; at a lift station %d (skier %d, fallen %d, standing %d)\n",
		c.N, c.SameGroup, c.AtStation, c.StationKind[ContactSkier], c.StationKind[ContactFallen], c.StationKind[ContactStanding])
	open := c.Kind[ContactSkier] + c.Kind[ContactFallen] + c.Kind[ContactStanding]
	fmt.Fprintf(out, "  on the snow %d: skier %d, fallen %d, standing %d; max closing %.1f m/s\n",
		open, c.Kind[ContactSkier], c.Kind[ContactFallen], c.Kind[ContactStanding], c.MaxClose)
	if open == 0 {
		return
	}
	fmt.Fprintf(out, "  closing <2 %d, 2-5 %d, 5-10 %d, 10+ %d; skier-on-skier same way %d, crossing %d, head on %d\n",
		c.Band[0], c.Band[1], c.Band[2], c.Band[3], c.Angle[ContactSameWay], c.Angle[ContactCrossing], c.Angle[ContactHeadOn])
	writeTop(out, "  standing:", c.Standing, func(k string) string { return k })
	names := map[uint64]string{0: "(off any run)"}
	for _, t := range w.Trails {
		names[t.ID] = t.Name
	}
	writeTop(out, "  runs:", c.ByTrail, func(id uint64) string { return names[id] })
	writeTop(out, "  spots:", c.Spots, func(k [2]int) string {
		return fmt.Sprintf("(%d,%d)", k[0]*contactSpot+contactSpot/2, k[1]*contactSpot+contactSpot/2)
	})
}

// writeTop prints a line of the eight biggest counts in m, most first.
func writeTop[K comparable](out io.Writer, label string, m map[K]int, name func(K) string) {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] })
	fmt.Fprint(out, label)
	for i, k := range keys {
		if i == 8 {
			break
		}
		sep := ","
		if i == 0 {
			sep = ""
		}
		fmt.Fprintf(out, "%s %s %d", sep, name(k), m[k])
	}
	fmt.Fprintln(out)
}

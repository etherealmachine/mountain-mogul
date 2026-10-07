package world

import (
	_ "embed"
	"math/rand"
	"strings"

	"mountain-mogul/internal/ai"
)

// DefaultGuestPoolSize is the catchment we seed into a fresh resort:
// 10k potential visitors is large enough that any plausible season
// turnover (a couple thousand visits) is a small fraction of the pool,
// so the same guest doesn't recur mechanically.
const DefaultGuestPoolSize = 10000

//go:embed names_first.txt
var firstNamesRaw string

//go:embed names_last.txt
var lastNamesRaw string

var (
	firstNames = splitNames(firstNamesRaw)
	lastNames  = splitNames(lastNamesRaw)
)

func splitNames(raw string) []string {
	parts := strings.Split(raw, "\n")
	out := parts[:0]
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SeedGuests fills w.Guests with `count` potential visitors. Each guest
// gets a random name, skill, and per-season visit frequency drawn from a
// long-tail distribution: most guests are casual (1–3 visits/season), a
// small minority are regulars (one visit every day or two). Everyone
// skis: snowboarders come with their own behaviour as DLC (Release), and
// Discipline stays so they can.
func SeedGuests(w *World, seed int64, count int) {
	if w == nil || count <= 0 {
		return
	}
	g := rand.New(rand.NewSource(seed))
	w.Guests = make([]*Guest, 0, count)
	for i := 0; i < count; i++ {
		w.Guests = append(w.Guests, newPoolGuest(w, g, 0))
	}
}

// newPoolGuest rolls one potential visitor who comes by road entry home
// (0 for none).
func newPoolGuest(w *World, g *rand.Rand, home uint64) *Guest {
	skill := rollSkill(g)
	traits := ai.TraitsFor(skill)
	traits.Tastes = RollTastes(skill, g)
	traits.DailyBudget = DailyBudgetFor(skill)
	return &Guest{
		ID:              w.NextID(),
		Name:            firstNames[g.Intn(len(firstNames))] + " " + lastNames[g.Intn(len(lastNames))],
		Discipline:      Ski,
		Traits:          traits,
		VisitsPerSeason: rollVisitsPerSeason(g),
		ArrivalOffset:   RollArrivalOffset(g),
		State:           AtHome,
		HomeEntryID:     home,
	}
}

// SyncGuestPool makes the guest pool match the road entries: each entry's
// Pool guests live that way (Guest.HomeEntryID) and arrive and leave by
// it. Guests with no matching entry fill entries that are short before
// new guests are rolled; entries with too many lose guests who are at
// home. A map without entries keeps a DefaultGuestPoolSize pool with no
// home entry. Run on load, so pools edited in the editor take effect.
func SyncGuestPool(w *World, seed int64) {
	entries := w.Entries()
	if len(entries) == 0 {
		if len(w.Guests) == 0 {
			SeedGuests(w, seed, DefaultGuestPoolSize)
		}
		return
	}
	valid := map[uint64]bool{}
	have := map[uint64]int{}
	for _, e := range entries {
		valid[e.ID] = true
	}
	var orphans []*Guest
	for _, g := range w.Guests {
		if valid[g.HomeEntryID] {
			have[g.HomeEntryID]++
		} else {
			orphans = append(orphans, g)
		}
	}
	for _, e := range entries {
		for have[e.ID] < e.Pool && len(orphans) > 0 {
			orphans[len(orphans)-1].HomeEntryID = e.ID
			orphans = orphans[:len(orphans)-1]
			have[e.ID]++
		}
		if n := e.Pool - have[e.ID]; n > 0 {
			r := rand.New(rand.NewSource(seed ^ int64(e.ID)*7919 ^ int64(have[e.ID])))
			for i := 0; i < n; i++ {
				w.Guests = append(w.Guests, newPoolGuest(w, r, e.ID))
			}
			have[e.ID] = e.Pool
		}
	}
	// Drop leftover orphans and any surplus, keeping guests who are out.
	extra := map[uint64]int{}
	for _, e := range entries {
		extra[e.ID] = have[e.ID] - e.Pool
	}
	kept := w.Guests[:0]
	for _, g := range w.Guests {
		drop := g.State == AtHome && (!valid[g.HomeEntryID] || extra[g.HomeEntryID] > 0)
		if drop && valid[g.HomeEntryID] {
			extra[g.HomeEntryID]--
		}
		if !drop {
			kept = append(kept, g)
		}
	}
	w.Guests = kept
}

// DailyBudgetFor is the dollars a guest of the given skill will spend on
// one visit (ticket, parking, food). Not persisted: save load derives it
// from skill again, so it must stay a pure function of skill. The floor
// covers a default day ticket, a parking share and a meal, so even the
// greenest beginner can eat at the resort.
func DailyBudgetFor(skill float32) float32 {
	return 90 + skill*150
}

// rollSkill biases toward beginners — the real-world resort split is
// roughly 60/30/10 beginner/intermediate/advanced. Returns a continuous
// value in [0, 1] drawn uniformly within the appropriate tier band.
func rollSkill(g *rand.Rand) float32 {
	r := g.Float32()
	switch {
	case r < 0.6:
		return SkillInTier(0, g)
	case r < 0.9:
		return SkillInTier(1, g)
	default:
		return SkillInTier(2, g)
	}
}

// RollArrivalOffset draws when a guest likes to arrive, in clock hours
// after opening: a quarter are eager, in the car park 1.5 to 2.5 hours
// early so they're lined up when the lifts start; half come in the
// morning, from an hour before opening to an hour and a half after; and
// a quarter come late, 1.5 to 4 hours after opening.
func RollArrivalOffset(g *rand.Rand) float32 {
	r := g.Float32()
	switch {
	case r < 0.25:
		return -2.5 + g.Float32()
	case r < 0.75:
		return -1 + 2.5*g.Float32()
	default:
		return 1.5 + 2.5*g.Float32()
	}
}

// tasteSpread is how far a guest's tastes stray from their archetype's
// centre (the standard deviation of each affinity).
const tasteSpread = 0.25

// RollTastes draws a guest's tastes: an archetype picked by how common it
// is at their skill tier, then each affinity around its centre, kept to
// -1..+1.
func RollTastes(skill float32, g *rand.Rand) ai.Tastes {
	tier := 0
	switch {
	case skill >= ai.SkillAdvancedThreshold:
		tier = 2
	case skill >= ai.SkillIntermediateThreshold:
		tier = 1
	}
	var total float32
	for _, a := range ai.Archetypes {
		total += a.Share[tier]
	}
	pick := g.Float32() * total
	arch := ai.Archetypes[0]
	for _, a := range ai.Archetypes {
		if pick < a.Share[tier] {
			arch = a
			break
		}
		pick -= a.Share[tier]
	}
	var t ai.Tastes
	for i, c := range arch.Centre {
		t[i] = max(-1, min(1, c+float32(g.NormFloat64())*tasteSpread))
	}
	return t
}

// SkillInTier draws a skill uniformly within a tier's band: 0 beginner,
// 1 intermediate, 2 advanced.
func SkillInTier(tier int, g *rand.Rand) float32 {
	switch tier {
	case 0:
		return g.Float32() * ai.SkillIntermediateThreshold
	case 1:
		return ai.SkillIntermediateThreshold + g.Float32()*(ai.SkillAdvancedThreshold-ai.SkillIntermediateThreshold)
	default:
		return ai.SkillAdvancedThreshold + g.Float32()*(1-ai.SkillAdvancedThreshold)
	}
}

func rollVisitsPerSeason(g *rand.Rand) float32 {
	r := g.Float32()
	switch {
	case r < 0.70:
		return 1 + g.Float32()*2
	case r < 0.90:
		return 3 + g.Float32()*5
	case r < 0.99:
		return 8 + g.Float32()*22
	default:
		return 90 + g.Float32()*90
	}
}

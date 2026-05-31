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
// gets a random name, discipline, skill, and per-season visit frequency
// drawn from a long-tail distribution: most guests are casual (1–3
// visits/season), a small minority are regulars (one visit every day or
// two). Snowboarders are ~20% of the catchment.
//
// Called by scene/scenario load paths for fresh worlds. Saved worlds
// rehydrate Guests from disk and skip this entirely.
func SeedGuests(w *World, seed int64, count int) {
	if w == nil || count <= 0 {
		return
	}
	g := rand.New(rand.NewSource(seed))
	w.Guests = make([]*Guest, 0, count)
	for i := 0; i < count; i++ {
		skill := rollSkill(g)
		disc := rollDiscipline(g)
		traits := ai.TraitsFor(skill)
		gladeProb := float32(0.0)
		if skill >= ai.SkillAdvancedThreshold {
			gladeProb = 0.30
		}
		traits.LikesGlades = g.Float32() < gladeProb
		traits.PrefersGroomed = g.Float32() < 0.60
		traits.DailyBudget = 40 + skill*160
		guest := &Guest{
			ID:              w.NextID(),
			Name:            firstNames[g.Intn(len(firstNames))] + " " + lastNames[g.Intn(len(lastNames))],
			Discipline:      disc,
			Traits:          traits,
			VisitsPerSeason: rollVisitsPerSeason(g),
			State:           AtHome,
		}
		w.Guests = append(w.Guests, guest)
	}
}

// rollSkill biases toward beginners — the real-world resort split is
// roughly 60/30/10 beginner/intermediate/advanced. Returns a continuous
// value in [0, 1] drawn uniformly within the appropriate tier band.
func rollSkill(g *rand.Rand) float32 {
	r := g.Float32()
	switch {
	case r < 0.6:
		return g.Float32() * ai.SkillIntermediateThreshold
	case r < 0.9:
		return ai.SkillIntermediateThreshold + g.Float32()*(ai.SkillAdvancedThreshold-ai.SkillIntermediateThreshold)
	default:
		return ai.SkillAdvancedThreshold + g.Float32()*(1-ai.SkillAdvancedThreshold)
	}
}

func rollDiscipline(g *rand.Rand) Discipline {
	if g.Float32() < 0.2 {
		return Snowboard
	}
	return Ski
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

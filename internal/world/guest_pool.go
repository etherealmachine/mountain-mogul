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
func SeedGuests(w *World, rng *rand.Rand, count int) {
	if w == nil || rng == nil || count <= 0 {
		return
	}
	w.Guests = make([]*Guest, 0, count)
	for i := 0; i < count; i++ {
		skill := rollSkill(rng)
		disc := rollDiscipline(rng)
		traits := ai.TraitsFor(skill)
		// LikesGlades is a minority taste — ~15 % of the catchment. Skews
		// slightly higher for advanced skiers since gladiated terrain is
		// effectively black-rated.
		gladeProb := float32(0.10)
		if skill == ai.SkillAdvanced {
			gladeProb = 0.30
		}
		traits.LikesGlades = rng.Float32() < gladeProb
		// PrefersGroomed is the modal preference at a real resort — most
		// visitors want corduroy. ~60 %.
		traits.PrefersGroomed = rng.Float32() < 0.60
		g := &Guest{
			ID:              w.NextID(),
			Name:            firstNames[rng.Intn(len(firstNames))] + " " + lastNames[rng.Intn(len(lastNames))],
			Discipline:      disc,
			Traits:          traits,
			VisitsPerSeason: rollVisitsPerSeason(rng),
			State:           AtHome,
		}
		w.Guests = append(w.Guests, g)
	}
}

// rollSkill biases toward beginners — the real-world resort split is
// roughly 60/30/10 beginner/intermediate/advanced. Tunable as we get
// more design clarity.
func rollSkill(rng *rand.Rand) ai.SkillLevel {
	r := rng.Float32()
	switch {
	case r < 0.6:
		return ai.SkillBeginner
	case r < 0.9:
		return ai.SkillIntermediate
	default:
		return ai.SkillAdvanced
	}
}

// rollDiscipline returns Ski 80% of the time, Snowboard 20%. Matches
// rough industry averages. (Once snowboard physics ships we can split
// the trait distribution per-discipline.)
func rollDiscipline(rng *rand.Rand) Discipline {
	if rng.Float32() < 0.2 {
		return Snowboard
	}
	return Ski
}

// rollVisitsPerSeason draws from a long-tail distribution:
//
//   - 70% casual:   1–3 visits/season (Uniform[1, 3])
//   - 20% tourist:  3–8 visits/season
//   - 9%  enthusiast: 8–30 visits/season
//   - 1%  regular:  ~90–180 visits/season (every day or two)
//
// Drives the demand poll's per-guest Bernoulli rate. Edge cases like
// "casual who happens to be an expert" fall out naturally — skill and
// frequency are sampled independently.
func rollVisitsPerSeason(rng *rand.Rand) float32 {
	r := rng.Float32()
	switch {
	case r < 0.70:
		return 1 + rng.Float32()*2 // 1–3
	case r < 0.90:
		return 3 + rng.Float32()*5 // 3–8
	case r < 0.99:
		return 8 + rng.Float32()*22 // 8–30
	default:
		return 90 + rng.Float32()*90 // 90–180
	}
}

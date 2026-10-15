package world

// Avalanche debris: the tumbled blocks a slide leaves where it stops.
// Cell.Debris is how much of the cell's surface snow (SWE, metres) is
// debris. It's drawn as a lumpy field of blocks (terrain.frag, the
// renderer's snow blocks) as deep as DebrisDepth, and outlasts a change
// of snow kind: new snow buries it, a thaw slumps it, and a cat grooming
// the cell clears it.

const (
	// DebrisBurySWE is the new snow, SWE, that buries debris out of
	// sight: about 80 cm of fresh powder.
	DebrisBurySWE = float32(0.1)
	// debrisKeepCold and debrisKeepWarm are the share of debris still
	// showing after a day: blocks round off and sink slowly in the cold,
	// faster in a thaw.
	debrisKeepCold = float32(0.97)
	debrisKeepWarm = float32(0.9)
	// debrisGone is the SWE below which debris no longer shows.
	debrisGone = float32(0.005)
)

// DebrisDepth is the depth of the cell's debris, metres.
func (c *Cell) DebrisDepth() float32 {
	return c.Debris / KindDensity(KindAvalancheDebris)
}

// AddDebris drops swe metres of avalanche snow on the cell as debris.
func (c *Cell) AddDebris(swe float32) {
	c.Top.Accumulation += swe
	c.Top.SetKind(KindAvalancheDebris)
	c.Top.Density = max(c.Top.Density, KindDensity(KindAvalancheDebris))
	c.Debris += swe
	c.Grooming = 0
	c.MogulSize = 0
}

// TakeDebris moves up to swe metres of the cell's debris away (it slumps
// to a neighbour), and returns how much it took.
func (c *Cell) TakeDebris(swe float32) float32 {
	swe = min(swe, c.Debris, c.Top.Accumulation)
	if swe <= 0 {
		return 0
	}
	c.Debris -= swe
	c.Top.Accumulation -= swe
	return swe
}

// BuryDebris covers the cell's debris with swe metres of new snow.
func (c *Cell) BuryDebris(swe float32) {
	if c.Debris > 0 {
		c.Debris *= 1 - min(max(swe/DebrisBurySWE, 0), 1)
		c.trimDebris()
	}
}

// SettleDebris is a day's slumping of the cell's debris, faster in a thaw.
func (c *Cell) SettleDebris(warm bool) {
	if c.Debris <= 0 {
		return
	}
	if warm {
		c.Debris *= debrisKeepWarm
	} else {
		c.Debris *= debrisKeepCold
	}
	c.trimDebris()
}

// trimDebris keeps debris within the snow there is, and drops a trace.
func (c *Cell) trimDebris() {
	c.Debris = min(c.Debris, c.TotalSWE())
	if c.Debris < debrisGone {
		c.Debris = 0
	}
}

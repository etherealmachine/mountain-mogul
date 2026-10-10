package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Ski racks: guests don't take skis indoors. On the way into a building
// a guest carrying skis leaves them in a rack near the door with room
// (BuildingSkiRack, placed on the snow), or sticks them in the snow by the
// door when there's none, and picks them up on the way out (sim: gear
// errands). Skis left out are drawn where they are (Guest.Stash).

const (
	// SkiRackCost is what a rack costs to place.
	SkiRackCost = 1_500
	// SkiRackPairs is how many pairs of skis a rack holds.
	SkiRackPairs = 10
	// SkiRackReach is how far from the door a guest is going in by
	// they'll walk to a rack, metres.
	SkiRackReach = 40.0
	// skiRackPitch is the spacing between pairs along the rack, and
	// skiRackFront how far in front of its rail they stand, metres
	// (models-src/ski_rack.scad).
	skiRackPitch = 0.3
	skiRackFront = 0.3
)

// SkiStash is where a guest's skis are while they aren't carrying them:
// in a rack's slot or stuck in the snow.
type SkiStash struct {
	// Out is set once the skis have left their hands.
	Out bool
	// RackID is the rack they're in or the guest is taking them to (with
	// Slot), 0 for the snow.
	RackID uint64
	Slot   int
	// Pos and Yaw are where the pair stands and which way it faces.
	Pos mgl32.Vec2
	Yaw float32
	// Lying is set when the pair lies flat on the snow, along Yaw from
	// Pos: dropped when the rack they were in was taken away.
	Lying bool
}

// CarriesSkis reports whether g has skis in hand: they brought or rented
// some, aren't wearing them, and haven't left them anywhere.
func (g *Guest) CarriesSkis() bool {
	return !g.NeedsGear && !g.SkisOn && !g.Stash.Out
}

// SkiRackSlot is where pair i stands in rack b, and which way it faces:
// in a row in front of the rail, leaning back on it.
func (b *Building) SkiRackSlot(i int) (mgl32.Vec2, float32) {
	ax, az := FootprintRect{Rotation: b.Rotation}.Axes()
	along := (float32(i) - float32(SkiRackPairs-1)/2) * skiRackPitch
	pos := b.Pos.Add(ax.Mul(along)).Add(az.Mul(skiRackFront))
	// Facing out from the rail: the pair's front is +az.
	return pos, float32(math.Atan2(float64(az[0]), float64(az[1])))
}

// dropRackSkis lets the skis in rack id fall when it's taken away: each
// pair tips forward off the rail and lies flat on the snow in front of
// its slot, a little askew. Guests still on their way to the rack pick
// another place to leave their skis.
func (w *World) dropRackSkis(id uint64) {
	for _, g := range w.OnMountain {
		if g.Stash.RackID != id {
			continue
		}
		if !g.Stash.Out {
			g.Stash = SkiStash{}
			continue
		}
		front := mgl32.Vec2{float32(math.Sin(float64(g.Stash.Yaw))), float32(math.Cos(float64(g.Stash.Yaw)))}
		// Askew by up to ±0.4 rad, fixed by the guest.
		skew := (float32(g.ID*0x9E3779B9%1000)/1000 - 0.5) * 0.8
		g.Stash = SkiStash{
			Out:   true,
			Pos:   g.Stash.Pos.Add(front.Mul(0.8)),
			Yaw:   g.Stash.Yaw + skew,
			Lying: true,
		}
	}
}

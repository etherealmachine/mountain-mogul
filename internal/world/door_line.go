package world

import (
	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/ai"
)

// Lines at a door: guests waiting for a seat or a turn at the counter
// (Visit.Waiting) stand single file straight out from the door they came
// to, and a long line snakes back and forth beside it, a row at a time,
// like a rope maze. Guests inside aren't on the map (Guest.Indoors).

const (
	doorLineSpacing = 1.0 // metres between guests in line
	doorLineStart   = 1.0 // the first guest's distance out from the wall
	doorLineRow     = 6   // guests in a row before the line turns back
	doorLineRowGap  = 1.2 // metres between rows
)

// DoorLineSlot is where the k-th guest (0 at the front) in the line at
// door i stands, and the way they face: toward the guest ahead, or the
// door. side (+1 or -1) is which side of the door the rows step to.
func (b *Building) DoorLineSlot(i, k int, side float32) (pos, face mgl32.Vec2) {
	pos = b.doorLinePoint(i, k, side)
	var ahead mgl32.Vec2
	if k > 0 {
		ahead = b.doorLinePoint(i, k-1, side)
	} else {
		d := b.Doors[i]
		ahead = b.GridToWorld((float32(d.Cell[0])+0.5)*CellSize, (float32(d.Cell[1])+0.5)*CellSize)
	}
	if f := ahead.Sub(pos); f.Len() > 1e-4 {
		face = f.Normalize()
	}
	return pos, face
}

// doorLinePoint is DoorLineSlot's position.
func (b *Building) doorLinePoint(i, k int, side float32) mgl32.Vec2 {
	d := b.Doors[i]
	dir := mgl32.Vec2{float32(d.Dir[0]), float32(d.Dir[1])}
	across := mgl32.Vec2{-dir[1], dir[0]}.Mul(side)
	wall := mgl32.Vec2{(float32(d.Cell[0]) + 0.5) * CellSize, (float32(d.Cell[1]) + 0.5) * CellSize}.Add(dir.Mul(CellSize / 2))
	row, j := k/doorLineRow, k%doorLineRow
	if row%2 == 1 {
		j = doorLineRow - 1 - j // coming back toward the wall
	}
	g := wall.Add(dir.Mul(doorLineStart + float32(j)*doorLineSpacing)).Add(across.Mul(float32(row) * doorLineRowGap))
	return b.GridToWorld(g[0], g[1])
}

// DoorLineSide is the side the line at door i snakes to: the one with
// more open, walkable ground for its first few rows.
func (b *Building) DoorLineSide(t *Terrain, i int) float32 {
	best, bestN := float32(1), -1
	for _, side := range [2]float32{1, -1} {
		n := 0
		for k := doorLineRow; k < 4*doorLineRow; k++ {
			p := b.doorLinePoint(i, k, side)
			cx, cz := int(p[0]/CellSize), int(p[1]/CellSize)
			if p[0] >= 0 && p[1] >= 0 && t.InBounds(cx, cz) && t.Cells[cx][cz].Walkable() {
				n++
			}
		}
		if n > bestN {
			best, bestN = side, n
		}
	}
	return best
}

// Indoors reports whether g is inside a building, using a service: not
// drawn, and not in anyone's way.
func (g *Guest) Indoors() bool {
	return g.RestTimer > 0 && !g.Visit.Waiting && g.Plan.Head().Kind == ai.ActUseService
}

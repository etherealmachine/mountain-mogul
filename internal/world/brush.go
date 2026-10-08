package world

// BrushCells returns all grid cells within a circular radius of (cx, cz).
// Radius is in cells (1 cell = CellSize metres).
func BrushCells(cx, cz, radius int) [][2]int {
	var out [][2]int
	r2 := radius * radius
	for dx := -radius; dx <= radius; dx++ {
		for dz := -radius; dz <= radius; dz++ {
			if dx*dx+dz*dz <= r2 {
				out = append(out, [2]int{cx + dx, cz + dz})
			}
		}
	}
	return out
}

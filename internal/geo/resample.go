package geo

// ResampleToGrid bilinearly resamples src (any dimensions) to destCols×destRows.
// Normalizes so the minimum elevation becomes 0, then divides by 10 so that
// 1 game unit ≈ 10 metres of real-world elevation gain.
func ResampleToGrid(src [][]float32, destCols, destRows int) [][]float32 {
	srcRows := len(src)
	if srcRows == 0 || destRows == 0 || destCols == 0 {
		return nil
	}
	srcCols := len(src[0])

	out := make([][]float32, destRows)
	for row := 0; row < destRows; row++ {
		out[row] = make([]float32, destCols)
		for col := 0; col < destCols; col++ {
			sr := float32(row) * float32(srcRows-1) / float32(destRows-1)
			sc := float32(col) * float32(srcCols-1) / float32(destCols-1)
			out[row][col] = bilinearSample(src, sr, sc)
		}
	}

	// Find min and max elevation across the output grid.
	minE, maxE := out[0][0], out[0][0]
	for _, row := range out {
		for _, e := range row {
			if e < minE {
				minE = e
			}
			if e > maxE {
				maxE = e
			}
		}
	}

	span := maxE - minE
	if span < 1 {
		span = 1
	}

	// Target max elevation = 30% of terrain horizontal extent (cellSize=10).
	targetHeight := float32(destCols) * 10.0 * 0.30
	scale := targetHeight / span
	// Cap amplification: don't scale more than what a 30m span would produce.
	// This prevents absurd exaggeration of nearly-flat terrain while allowing
	// real mountains to reach the full targetHeight.
	maxScale := targetHeight / 30.0
	if scale > maxScale {
		scale = maxScale
	}
	if scale < 0.05 {
		scale = 0.05 // don't compress massive mountains into nothing
	}

	for row := range out {
		for col := range out[row] {
			out[row][col] = (out[row][col] - minE) * scale
		}
	}
	return out
}

func bilinearSample(grid [][]float32, r, c float32) float32 {
	rows := len(grid)
	cols := len(grid[0])

	r0 := int(r)
	c0 := int(c)
	r1 := r0 + 1
	c1 := c0 + 1
	if r1 >= rows {
		r1 = rows - 1
	}
	if c1 >= cols {
		c1 = cols - 1
	}

	fr := r - float32(r0)
	fc := c - float32(c0)

	v00 := grid[r0][c0]
	v10 := grid[r1][c0]
	v01 := grid[r0][c1]
	v11 := grid[r1][c1]

	return v00*(1-fr)*(1-fc) + v10*fr*(1-fc) + v01*(1-fr)*fc + v11*fr*fc
}

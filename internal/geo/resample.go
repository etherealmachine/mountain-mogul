package geo

// bilinearSample reads grid[r][c] at fractional row r and column c.
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

package world

import "github.com/go-gl/mathgl/mgl32"

// Tunnels (notes/next/Trains and Tunnels.md): where a line would have to
// cut deeper than TunnelDepth into a hill, it goes into a tunnel instead:
// the hill stays, portals stand at each end, and what's inside is hidden.
// The ground can't have holes (it's a heightmap), so a tunnel shows as
// its portals and a dashed line over its course.
const (
	// TunnelDepth is the cut, metres of ground over the track, from
	// which a line tunnels; a tunnel shorter than TunnelMinLength isn't
	// worth it (the line cuts instead), and gaps under TunnelJoin
	// between two join them.
	TunnelDepth     = float32(12)
	TunnelMinLength = float32(30)
	TunnelJoin      = float32(40)
	// TunnelCostPerM is a tunnel's price a metre, in place of the
	// track's.
	TunnelCostPerM = 8_000
)

// markTunnels marks the track samples of line l that run under the hill.
func (l *RailLine) markTunnels(t *Terrain) {
	n := len(l.Track)
	deep := make([]bool, n)
	for i, s := range l.Track {
		g := t.InterpolatedGroundElevationAt(s.Pos[0], s.Pos[1])
		deep[i] = g-s.Y > TunnelDepth
	}
	// Join near runs, then drop short ones.
	runs := runsOf(deep)
	var joined [][2]int
	for _, r := range runs {
		if k := len(joined); k > 0 && l.Track[r[0]].Dist-l.Track[joined[k-1][1]].Dist < TunnelJoin {
			joined[k-1][1] = r[1]
			continue
		}
		joined = append(joined, r)
	}
	for i := range l.Track {
		l.Track[i].Tunnel = false
	}
	for _, r := range joined {
		if l.Track[r[1]].Dist-l.Track[r[0]].Dist < TunnelMinLength {
			continue
		}
		for i := r[0]; i <= r[1]; i++ {
			l.Track[i].Tunnel = true
		}
	}
}

// runsOf is the [first, last] index of each run of trues.
func runsOf(b []bool) [][2]int {
	var out [][2]int
	for i := 0; i < len(b); i++ {
		if !b[i] {
			continue
		}
		j := i
		for j+1 < len(b) && b[j+1] {
			j++
		}
		out = append(out, [2]int{i, j})
		i = j
	}
	return out
}

// TunnelPortal is one end of a tunnel: where it is, its height, and its
// heading into the hill (as Guest.Heading).
type TunnelPortal struct {
	Pos     mgl32.Vec2
	Y       float32
	Heading float32
}

// Portals is line l's tunnel mouths.
func (l *RailLine) Portals() []TunnelPortal {
	var out []TunnelPortal
	for i := 1; i < len(l.Track); i++ {
		a, b := l.Track[i-1], l.Track[i]
		if a.Tunnel == b.Tunnel {
			continue
		}
		// Facing into the tunnel: along the line where it starts, back
		// where it ends.
		d := (a.Dist + b.Dist) / 2
		pos, y, heading, _ := l.At(d)
		if a.Tunnel {
			heading += 3.14159265
		}
		out = append(out, TunnelPortal{Pos: pos, Y: y, Heading: heading})
	}
	return out
}

// InTunnel reports whether d metres along line l is in a tunnel.
func (l *RailLine) InTunnel(d float32) bool {
	if len(l.Track) == 0 {
		return false
	}
	i := min(max(int(d/RailSampleStep+0.5), 0), len(l.Track)-1)
	return l.Track[i].Tunnel
}

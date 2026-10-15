package render

import (
	"fmt"
	"sort"
	"strings"

	"github.com/go-gl/gl/v4.1-core/gl"
)

// GPU pass timing (a dev tool: -gpu-passes): an elapsed-time query a
// pass, one after another (macOS's OpenGL has no timestamps), read back
// at the frame's end, their medians reported. Off, gpuMark costs nothing.

type gpuPasses struct {
	on      bool
	queries []uint32
	names   []string
	n       int
	samples map[string][]float64
	order   []string
}

// EnableGPUPasses turns pass timing on.
func (r *Renderer) EnableGPUPasses() {
	r.gpuPasses = &gpuPasses{on: true, samples: map[string][]float64{}}
}

// gpuMark ends the pass before and starts the one named name.
func (r *Renderer) gpuMark(name string) {
	g := r.gpuPasses
	if g == nil || !g.on {
		return
	}
	if r.terrainB != nil && name != "" && name != "frame A" && name != "frame B" {
		return // A/B: whole frames only
	}
	if g.n > 0 {
		gl.EndQuery(gl.TIME_ELAPSED)
	}
	if name == "" {
		return
	}
	if g.n == len(g.queries) {
		var q uint32
		gl.GenQueries(1, &q)
		g.queries = append(g.queries, q)
		g.names = append(g.names, "")
	}
	gl.BeginQuery(gl.TIME_ELAPSED, g.queries[g.n])
	g.names[g.n] = name
	g.n++
}

// GPUPassesFrameEnd closes the frame's last pass and records the
// frame's times.
func (r *Renderer) GPUPassesFrameEnd() {
	g := r.gpuPasses
	if g == nil || !g.on {
		return
	}
	if g.n == 0 {
		return
	}
	r.gpuMark("")
	for i := 0; i < g.n; i++ {
		var t uint64
		gl.GetQueryObjectui64v(g.queries[i], gl.QUERY_RESULT, &t)
		name := g.names[i]
		if _, ok := g.samples[name]; !ok {
			g.order = append(g.order, name)
		}
		g.samples[name] = append(g.samples[name], float64(t)/1e6)
	}
	g.n = 0
}

// GPUPassesReport is each pass's median GPU time, ms.
func (r *Renderer) GPUPassesReport() string {
	g := r.gpuPasses
	if g == nil {
		return ""
	}
	var b strings.Builder
	total := 0.0
	for _, name := range g.order {
		v := append([]float64(nil), g.samples[name]...)
		sort.Float64s(v)
		m := v[len(v)/2]
		total += m
		fmt.Fprintf(&b, "  %-14s %6.2f ms\n", name, m)
	}
	fmt.Fprintf(&b, "  %-14s %6.2f ms\n", "sum", total)
	return b.String()
}

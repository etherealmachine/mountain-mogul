package scene

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"

	"mountain-mogul/internal/render"
	"mountain-mogul/internal/settings"
	"mountain-mogul/internal/ui"
	"mountain-mogul/internal/world"
)

// The Rail tool (notes/next/Trains and Tunnels.md): lay a railway of the
// chosen style point by point, starting at the map's edge; clicking the
// last point again builds it, its station at that end. A ghost of the
// laid-out line follows the mouse, red where it can't be built.

// railTool is the Rail tool's session.
type railTool struct {
	style world.RailStyle
	nodes []mgl32.Vec2
	// ghostAt is the hover the ghost was last built for.
	ghostAt   mgl32.Vec2
	ghostN    int
	ghostCost int
}

// railFinishReach is how near the last point a click counts as
// clicking it again (to build).
const railFinishReach = 8

// startRailTool picks style and turns the Rail tool on.
func (s *Scenario) startRailTool(style world.RailStyle) {
	if s.activeTool == toolRail && s.railTool.style == style {
		s.setTool(toolRail) // off
		return
	}
	s.railTool = railTool{style: style}
	if s.activeTool != toolRail {
		s.setTool(toolRail)
	}
	s.setToast("Start at the map's edge; click to lay track; click the last point again to build the station there")
}

// railClick handles a click with the Rail tool at world (wx, wz).
func (s *Scenario) railClick(r *render.Renderer, wx, wz float32) {
	w := s.world
	p := mgl32.Vec2{wx, wz}
	rt := &s.railTool
	if len(rt.nodes) == 0 {
		if !w.NearMapEdge(p) {
			s.setToast("A railway comes in from beyond the map: start it at the edge")
			return
		}
		rt.nodes = append(rt.nodes, p)
		return
	}
	last := rt.nodes[len(rt.nodes)-1]
	if p.Sub(last).Len() > railFinishReach || len(rt.nodes) < 2 {
		rt.nodes = append(rt.nodes, p)
		return
	}
	// Build it.
	spec := rt.style.Spec()
	cost := world.RailLineCost(s.railPreview(rt.nodes))
	if problem := railProblem(rt.style, rt.nodes); problem != "" {
		s.setToast(problem)
		return
	}
	if !w.CanAfford(cost) {
		s.setToast(fmt.Sprintf("Need %s for this railway: short by %s",
			settings.FormatDollars(cost), settings.FormatDollars(cost-w.Available())))
		return
	}
	w.Spend(cost)
	w.PlaceRailLine(rt.style, rt.nodes)
	r.SetRailGhost(nil, nil, [3]float32{})
	r.FlushTerrainVerts(w.Terrain)
	r.RebuildStaticBatch(w)
	s.setToast(fmt.Sprintf("%s built: %s", spec.Label, settings.FormatDollars(cost)))
	*rt = railTool{style: rt.style}
}

// railProblem says why a railway through nodes can't be built, "" if it
// can.
func railProblem(style world.RailStyle, nodes []mgl32.Vec2) string {
	var m float32
	for i := 1; i < len(nodes); i++ {
		m += nodes[i].Sub(nodes[i-1]).Len()
	}
	if need := style.Spec().PlatformLength() + 60; m < need {
		return fmt.Sprintf("Too short: a %s needs at least %.0f m of track", style.Spec().Label, need)
	}
	return ""
}

// updateRailGhost shows the line as it would be laid with the mouse at
// the next point.
func (s *Scenario) updateRailGhost(r *render.Renderer) {
	rt := &s.railTool
	if len(rt.nodes) == 0 || !s.hoverValid {
		if rt.ghostN != 0 {
			r.SetRailGhost(nil, nil, [3]float32{})
			rt.ghostN = 0
		}
		return
	}
	hover := mgl32.Vec2{s.hoverWorld[0], s.hoverWorld[2]}
	nodes := append([]mgl32.Vec2(nil), rt.nodes...)
	if hover.Sub(nodes[len(nodes)-1]).Len() > railFinishReach {
		nodes = append(nodes, hover)
	}
	if len(nodes) == rt.ghostN && hover.Sub(rt.ghostAt).Len() < 1 {
		return
	}
	rt.ghostAt, rt.ghostN = hover, len(nodes)
	if len(nodes) < 2 {
		r.SetRailGhost(nil, nil, [3]float32{})
		return
	}
	l := s.railPreview(nodes)
	tint := ghostTint(s.world.CanAfford(world.RailLineCost(l)), railProblem(rt.style, nodes) == "")
	rt.ghostCost = world.RailLineCost(l)
	r.SetRailGhost(l, s.world.Terrain, tint)
}

// railCost is the cost preview for the Rail tool: the line as the ghost
// last laid it out.
func (s *Scenario) railCost() int { return s.railTool.ghostCost }

// railPreview lays out a line of the tool's style through nodes, without
// touching the ground.
func (s *Scenario) railPreview(nodes []mgl32.Vec2) *world.RailLine {
	l := &world.RailLine{Style: s.railTool.style, Nodes: nodes}
	l.LayOut(s.world.Terrain)
	return l
}

// openStationPopup shows a railway's station: its style, how often its
// trains run, where its train is, and who it's bringing.
func (s *Scenario) openStationPopup(l *world.RailLine, screenW, screenH int) {
	spec := l.Style.Spec()
	w := ui.NewWindow(spec.Label, 0, 0)
	w.AddLabel("Train", func() string {
		return fmt.Sprintf("%d riders, %.0f km/h, %s a trip", spec.Capacity, spec.Speed*3.6, settings.FormatDollars(spec.TripCost))
	})
	w.AddIntStepperFn("Trains every",
		func() string { return fmt.Sprintf("%.0f minutes", l.Interval) },
		func() { l.Interval = max(l.Interval/2, 15) },
		func() { l.Interval = min(l.Interval*2, 240) })
	w.AddLabel("Now", func() string {
		switch l.Train.State {
		case world.TrainInbound:
			return "the train's coming in"
		case world.TrainDwelling:
			return "the train's at the platform"
		case world.TrainOutbound:
			return "the train's leaving"
		}
		return "the next train's on its way"
	})
	w.AddLabel("Booked", func() string {
		n := 0
		for _, g := range l.Bound {
			n += len(g)
		}
		return fmt.Sprintf("%d guests on the next train", n)
	})
	w.AddLabel("Here by train", func() string {
		n := 0
		for _, g := range s.world.OnMountain {
			if g.CarLot == l.StationID {
				n++
			}
		}
		return fmt.Sprintf("%d on the mountain", n)
	})
	w.Visible = true
	s.placePopup(w, screenW, screenH)
	s.popup = w
}

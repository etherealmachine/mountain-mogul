package world

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Stations, networks, routes and trains (notes/next/Rail Network.md).
//
// A station stands beside a track, its platform along it. The tracks
// joined to it make its network, and the network's train is its best
// style anywhere: any high-speed track means a high-speed set, any
// electric an electric unit, else steam. Every Interval a train sets off
// for each station: with two or more entries on its network, through it
// from one entry to another the shortest way (the other way next time);
// with one, from it and back. It stops at every station it passes, and
// runs at each stretch's speed. An electric or high-speed train can't run
// on steam track, so its routes go round it, or there's no service.

// RailStation is a station: its building (BuildingStation, the same ID),
// where its guests arrive and leave, and where it stands on its track.
type RailStation struct {
	ID uint64
	// TrackID is its track and Dist its platform's middle along it,
	// found from Pos (the platform's middle) when tracks change
	// (PlaceStations); Side is which side of the track the platform is
	// (+1 left of the track's way, -1 right).
	TrackID uint64
	Dist    float32
	Pos     mgl32.Vec2
	Side    float32
	// Interval is the clock minutes between its trains.
	Interval float32
	// NextAt is the sim time its next train sets off, Flip which way a
	// through train runs next, and Bound the guests booked to come (not
	// saved).
	NextAt float64
	Flip   bool
	Bound  [][]*Guest
}

// StationPlatformSide is how far a platform's middle is off the track.
const StationPlatformSide = float32(3.45)

// RailStationByID is station id, or nil.
func (w *World) RailStationByID(id uint64) *RailStation {
	for _, st := range w.RailStations {
		if st.ID == id {
			return st
		}
	}
	return nil
}

// PlaceStations puts each station on the track nearest its platform's
// middle (its own, if it's still there), after tracks change; a station
// with no track near is left off any (TrackID 0: no trains).
func (w *World) PlaceStations() {
	for _, st := range w.RailStations {
		st.TrackID = 0
		if tr, i, _ := w.TrackNear(st.Pos, RailTrackReach); tr != nil {
			st.TrackID, st.Dist = tr.ID, tr.Track[i].Dist
			if b := w.findBuilding(st.ID); b != nil {
				b.Pos = w.PlatformSpot(st) // its guests' spot follows the platform
			}
		}
	}
}

// StationFrame is where station st stands: its platform's middle on the
// track, the track's height and heading there, and the platform's side
// as a unit vector; ok false if it's off any track.
func (w *World) StationFrame(st *RailStation) (pos mgl32.Vec2, y, heading float32, side mgl32.Vec2, ok bool) {
	tr := w.RailTrackByID(st.TrackID)
	if tr == nil || len(tr.Track) < 2 {
		return
	}
	pos, y, heading, _ = tr.At(st.Dist)
	s, c := float32(math.Sin(float64(heading))), float32(math.Cos(float64(heading)))
	side = mgl32.Vec2{c, -s}.Mul(st.Side) // left of the way, for +1
	return pos, y, heading, side, true
}

// PlatformSpot is where station st's arriving guests step off: the
// platform's middle.
func (w *World) PlatformSpot(st *RailStation) mgl32.Vec2 {
	pos, _, _, side, ok := w.StationFrame(st)
	if !ok {
		return st.Pos
	}
	return pos.Add(side.Mul(StationPlatformSide))
}

// RemoveStation removes station id and its building.
func (w *World) RemoveStation(id uint64) {
	for i, st := range w.RailStations {
		if st.ID == id {
			w.RailStations = append(w.RailStations[:i], w.RailStations[i+1:]...)
			break
		}
	}
	for i, b := range w.Buildings {
		if b.ID == id {
			w.Buildings = append(w.Buildings[:i], w.Buildings[i+1:]...)
			break
		}
	}
	w.RailRev++
}

// RailNetwork is a connected piece of the railway: its tracks, its
// entries, and its train's style (its best track's).
type RailNetwork struct {
	Tracks  []*RailTrack
	Entries []*RailJoint
	Train   RailStyle
}

// RailNetworks is the railway's networks, and which each track is in.
func (w *World) RailNetworks() ([]*RailNetwork, map[uint64]*RailNetwork) {
	of := map[uint64]*RailNetwork{}
	var nets []*RailNetwork
	for _, start := range w.RailTracks {
		if of[start.ID] != nil {
			continue
		}
		n := &RailNetwork{}
		nets = append(nets, n)
		seen := map[uint64]bool{}
		queue := []*RailTrack{start}
		of[start.ID] = n
		for len(queue) > 0 {
			tr := queue[0]
			queue = queue[1:]
			n.Tracks = append(n.Tracks, tr)
			n.Train = max(n.Train, tr.Style)
			for _, id := range tr.Ends {
				if seen[id] {
					continue
				}
				seen[id] = true
				if j := w.RailJointByID(id); j != nil && j.Entry {
					n.Entries = append(n.Entries, j)
				}
				for _, o := range w.TracksAt(id) {
					if of[o.ID] == nil {
						of[o.ID] = n
						queue = append(queue, o)
					}
				}
			}
		}
	}
	return nets, of
}

// Runs reports whether a train of style can run on track tr: anything
// on high-speed or electric track, and on steam track only a steam
// train (it has no wire).
func (tr *RailTrack) Runs(train RailStyle) bool {
	return train == RailSteam || tr.Style != RailSteam
}

// RouteLeg is a route's run along one track, from From to To metres along
// it (To < From going against the track's way).
type RouteLeg struct {
	Track    *RailTrack
	From, To float32
}

// RouteStop is a station on a route, and how far along the route its
// platform's middle is.
type RouteStop struct {
	Station *RailStation
	At      float32
}

// RailRoute is the way a train runs: its legs, its length, and its stops
// in order; Back is an out-and-back route, whose second half retraces the
// first.
type RailRoute struct {
	Legs   []RouteLeg
	Length float32
	Stops  []RouteStop
	Back   bool
}

// At is where on route r s metres along it is: the position, height, and
// heading of travel.
func (r *RailRoute) At(s float32) (pos mgl32.Vec2, y, heading, pitch float32, tunnel bool) {
	for i, l := range r.Legs {
		n := abs32(l.To - l.From)
		if s > n && i < len(r.Legs)-1 {
			s -= n
			continue
		}
		s = min(max(s, 0), n)
		d := l.From + s
		if l.To < l.From {
			d = l.From - s
		}
		pos, y, heading, pitch = l.Track.At(d)
		if l.To < l.From {
			heading += math.Pi
			pitch = -pitch
		}
		return pos, y, heading, pitch, l.Track.InTunnel(d)
	}
	return
}

// TrackAt is the track route r runs on s metres along it.
func (r *RailRoute) TrackAt(s float32) *RailTrack {
	for i, l := range r.Legs {
		n := abs32(l.To - l.From)
		if s <= n || i == len(r.Legs)-1 {
			return l.Track
		}
		s -= n
	}
	return nil
}

// StationRoute is the route a train for station st runs (flip for a
// through train the other way), the style of train, and if there's none,
// why.
func (w *World) StationRoute(st *RailStation, flip bool) (*RailRoute, RailStyle, string) {
	tr := w.RailTrackByID(st.TrackID)
	if tr == nil {
		return nil, 0, "not on any track"
	}
	_, of := w.RailNetworks()
	net := of[tr.ID]
	train := net.Train
	if len(net.Entries) == 0 {
		return nil, train, "no track to the map's edge"
	}
	if !tr.Runs(train) {
		return nil, train, "on steam track: the " + lowerFirst(train.Spec().Label) + "'s trains need it electrified"
	}
	// Shortest ways from each end of the station's track, not along it,
	// over track the train can run on.
	d0, via0 := w.railDistances(tr.Ends[0], tr, train)
	d1, via1 := w.railDistances(tr.Ends[1], tr, train)
	length := tr.Length()
	var best *RailRoute
	bestLen := float32(math.MaxFloat32)
	for _, e0 := range net.Entries {
		for _, e1 := range net.Entries {
			a, okA := d0[e0.ID]
			b, okB := d1[e1.ID]
			if e0 == e1 || !okA || !okB || a+length+b >= bestLen {
				continue
			}
			bestLen = a + length + b
			r := &RailRoute{}
			r.Legs = append(r.Legs, reverseLegs(w.railPath(via0, e0.ID))...)
			r.Legs = append(r.Legs, RouteLeg{Track: tr, From: 0, To: length})
			r.Legs = append(r.Legs, w.railPath(via1, e1.ID)...)
			best = r
		}
	}
	if best != nil {
		if flip {
			best.Legs = reverseLegs(best.Legs)
		}
	} else {
		// Out and back, from the nearest entry either way, running half a
		// train past the platform's middle to stop and turn round.
		var legs []RouteLeg
		bestLen = math.MaxFloat32
		half := train.Spec().TrainLength() / 2
		for _, e := range net.Entries {
			if a, ok := d0[e.ID]; ok && a+st.Dist < bestLen {
				bestLen = a + st.Dist
				legs = append(reverseLegs(w.railPath(via0, e.ID)), RouteLeg{Track: tr, From: 0, To: min(st.Dist+half, length)})
			}
			if b, ok := d1[e.ID]; ok && b+length-st.Dist < bestLen {
				bestLen = b + length - st.Dist
				legs = append(reverseLegs(w.railPath(via1, e.ID)), RouteLeg{Track: tr, From: length, To: max(st.Dist-half, 0)})
			}
		}
		if legs == nil {
			return nil, train, "steam track between it and the map's edge: the " + lowerFirst(train.Spec().Label) + "'s trains need it electrified"
		}
		best = &RailRoute{Legs: append(legs, reverseLegs(legs)...), Back: true}
	}
	for _, l := range best.Legs {
		best.Length += abs32(l.To - l.From)
	}
	w.routeStops(best)
	return best, train, ""
}

// railDistances is the shortest distance over track a train of style
// can run on from joint from to every joint, not using track skip, and
// the track and joint each was reached by.
func (w *World) railDistances(from uint64, skip *RailTrack, train RailStyle) (map[uint64]float32, map[uint64][2]uint64) {
	dist := map[uint64]float32{from: 0}
	via := map[uint64][2]uint64{}
	done := map[uint64]bool{}
	for {
		var cur uint64
		best := float32(math.MaxFloat32)
		for id, d := range dist {
			if !done[id] && d < best {
				cur, best = id, d
			}
		}
		if cur == 0 {
			return dist, via
		}
		done[cur] = true
		for _, tr := range w.TracksAt(cur) {
			if tr == skip || !tr.Runs(train) {
				continue
			}
			other := tr.Ends[0]
			if other == cur {
				other = tr.Ends[1]
			}
			if d, ok := dist[other]; !ok || best+tr.Length() < d {
				dist[other] = best + tr.Length()
				via[other] = [2]uint64{tr.ID, cur}
			}
		}
	}
}

// railPath is the legs from the start of a railDistances search to joint
// to, in order from the start.
func (w *World) railPath(via map[uint64][2]uint64, to uint64) []RouteLeg {
	var legs []RouteLeg
	for {
		v, ok := via[to]
		if !ok {
			break
		}
		tr := w.RailTrackByID(v[0])
		// From joint v[1] to joint to along tr.
		if tr.Ends[0] == v[1] {
			legs = append(legs, RouteLeg{Track: tr, From: 0, To: tr.Length()})
		} else {
			legs = append(legs, RouteLeg{Track: tr, From: tr.Length(), To: 0})
		}
		to = v[1]
	}
	// Collected backwards: put them in order.
	for i, j := 0, len(legs)-1; i < j; i, j = i+1, j-1 {
		legs[i], legs[j] = legs[j], legs[i]
	}
	return legs
}

// reverseLegs is legs run the other way.
func reverseLegs(legs []RouteLeg) []RouteLeg {
	out := make([]RouteLeg, len(legs))
	for i, l := range legs {
		out[len(legs)-1-i] = RouteLeg{Track: l.Track, From: l.To, To: l.From}
	}
	return out
}

// routeStops finds the stations along route r: each once, where it's
// first passed (an out-and-back route's on the way in, and its turn).
func (w *World) routeStops(r *RailRoute) {
	seen := map[uint64]bool{}
	var s float32
	half := r.Length
	if r.Back {
		half = r.Length / 2
	}
	for _, l := range r.Legs {
		n := abs32(l.To - l.From)
		for _, st := range w.RailStations {
			if st.TrackID != l.Track.ID || seen[st.ID] {
				continue
			}
			lo, hi := min(l.From, l.To), max(l.From, l.To)
			if st.Dist < lo-0.01 || st.Dist > hi+0.01 {
				continue
			}
			at := s + abs32(st.Dist-l.From)
			if at > half+0.01 {
				continue
			}
			seen[st.ID] = true
			r.Stops = append(r.Stops, RouteStop{Station: st, At: at})
		}
		s += n
	}
	// In order along the route.
	for i := 1; i < len(r.Stops); i++ {
		for j := i; j > 0 && r.Stops[j].At < r.Stops[j-1].At; j-- {
			r.Stops[j], r.Stops[j-1] = r.Stops[j-1], r.Stops[j]
		}
	}
}

// TrackSign is what a track shows over it: a warning on steam track in a
// network whose trains need electrified track, a slow sign on electric
// track in a high-speed network.
type TrackSign uint8

const (
	SignNone TrackSign = iota
	SignWarning
	SignSlow
)

// TrackSigns is each track's sign.
func (w *World) TrackSigns() map[uint64]TrackSign {
	_, of := w.RailNetworks()
	out := map[uint64]TrackSign{}
	for _, tr := range w.RailTracks {
		n := of[tr.ID]
		switch {
		case n.Train >= RailElectric && tr.Style == RailSteam:
			out[tr.ID] = SignWarning
		case n.Train == RailHighSpeed && tr.Style == RailElectric:
			out[tr.ID] = SignSlow
		}
	}
	return out
}

// Train is one train on its way: its route and style, where its front is
// along the route and how fast it's going, the stop it's making for next
// and whether it's standing there, and its seats left.
type Train struct {
	Route      *RailRoute
	Style      RailStyle
	Front      float32
	Speed      float32
	Stop       int
	Dwelling   bool
	DwellUntil float64
	Seats      int
	// Reversed is an out-and-back train on its way back (the engine at
	// the other end, as it's drawn).
	Reversed bool
}

func (w *World) findBuilding(id uint64) *Building {
	for _, b := range w.Buildings {
		if b.ID == id {
			return b
		}
	}
	return nil
}

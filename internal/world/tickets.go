package world

// Ticket tiers: besides the adult day ticket (World.DayTicketPrice) the
// player may offer a ticket for little kids (0–4), one for children
// (5–12), an afternoon ticket for guests arriving from AfternoonFrom, and
// a night ticket from NightFrom when there's a night session;
// each is a checkbox and a price at the ticket window. A guest pays the
// cheapest they qualify for.

// TicketTiers are the cheaper tickets on offer and their prices.
type TicketTiers struct {
	Kids, Child, Afternoon, Night                     bool
	KidsPrice, ChildPrice, AfternoonPrice, NightPrice int
}

// AfternoonFrom is the clock hour from which an arriving guest can buy an
// afternoon ticket: 12:30.
const AfternoonFrom = 12.5

// NightFrom is the clock hour from which an arriving guest can buy a
// night ticket: 16:00, for a night session (lights.go).
const NightFrom = 16.0

// Kids' ages: a little kid is LittleKidAge or younger, a child up to
// ChildAge.
const (
	LittleKidAge = 4
	ChildAge     = 12
)

// DefaultTicketTiers is a new game's tiers for an adult price of adult:
// little kids free, children at three quarters (to the nearest $5), the
// afternoon at half; the kids' tickets on offer, the afternoon not.
func DefaultTicketTiers(adult int) TicketTiers {
	return TicketTiers{
		Kids: true, Child: true,
		KidsPrice:      0,
		ChildPrice:     (adult*3 + 10) / 20 * 5, // three quarters, to the nearest $5
		AfternoonPrice: adult / 2,
		Night:          true,
		NightPrice:     adult / 2,
	}
}

// TicketPrice is the day ticket g pays arriving at clock hour hour: the
// adult price, or the cheapest tier they qualify for.
func (w *World) TicketPrice(g *Guest, hour float64) int {
	p := max(w.DayTicketPrice, 0)
	t := w.Tickets
	if t.Afternoon && hour >= AfternoonFrom {
		p = min(p, max(t.AfternoonPrice, 0))
	}
	if t.Night && hour >= NightFrom && w.NightSession() {
		p = min(p, max(t.NightPrice, 0))
	}
	if g.Child {
		switch {
		case g.Age <= LittleKidAge && t.Kids:
			p = min(p, max(t.KidsPrice, 0))
		case g.Age > LittleKidAge && t.Child:
			p = min(p, max(t.ChildPrice, 0))
		}
	}
	return p
}

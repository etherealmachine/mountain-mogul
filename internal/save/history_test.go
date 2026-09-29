package save

import (
	"testing"
	"time"

	"mountain-mogul/internal/world"
)

func TestHistoryBreakdownRoundTrip(t *testing.T) {
	src := world.NewHistory()
	sample := world.DailySample{
		Day:     time.Date(2026, 12, 9, 0, 0, 0, 0, time.UTC),
		Cash:    5000,
		Revenue: 700,
		Costs:   450,
		Open:    true,
	}
	sample.RevenueByKind[world.RevenueDayTickets] = 600
	sample.RevenueByKind[world.RevenueParking] = 100
	sample.CostsByKind[world.CostLifts] = 300
	sample.CostsByKind[world.CostInterest] = 150
	src.Push(sample)
	src.RecordRevenue(world.RevenueHeli, 80)

	dst := historyFromData(historyToData(src))
	got := dst.Ordered()
	if len(got) != 1 {
		t.Fatalf("got %d samples, want 1", len(got))
	}
	g := got[0]
	if g.RevenueByKind != sample.RevenueByKind || g.CostsByKind != sample.CostsByKind || !g.Open {
		t.Fatalf("sample = %+v, want breakdown %v / %v, open", g, sample.RevenueByKind, sample.CostsByKind)
	}
	if dst.RevenueToday != 80 || dst.RevenueByKindToday[world.RevenueHeli] != 80 {
		t.Fatalf("in-progress revenue = %d / %v, want 80 heli", dst.RevenueToday, dst.RevenueByKindToday)
	}

	// Samples from saves without a breakdown load with zero categories.
	old := historyFromData(&HistoryData{Samples: []DailySampleData{{Revenue: 50, Costs: 20}}})
	if s := old.Ordered()[0]; s.Revenue != 50 || s.RevenueByKind != [world.RevenueKindCount]int{} {
		t.Fatalf("legacy sample = %+v", s)
	}
}

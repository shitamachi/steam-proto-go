package collectionfacts

import "testing"

// TestSnapshotValidateDomainErrors exercises the per-domain rejection branches of
// Snapshot.Validate that the happy-path boundary test does not reach.
func TestSnapshotValidateDomainErrors(t *testing.T) {
	zero := int64(0)
	chart100 := func() []ChartEntry {
		out := []ChartEntry{}
		for i := 1; i <= 100; i++ {
			out = append(out, ChartEntry{Rank: int32(i), Type: "app", ID: int64(i), Title: "G", URL: "https://store.steampowered.com/app/1/"})
		}
		return out
	}
	cases := []struct {
		name string
		q    Query
		s    Snapshot
	}{
		{"invalid query", Query{Kind: "unknown"}, Snapshot{Player: &Player{Count: 1}}},
		{"news page bounds", Query{Kind: "news", AppID: 10}, Snapshot{News: &NewsPage{NextBefore: -1}}},
		{"incomplete definitions", Query{Kind: "achievements", AppID: 10, Language: "english"}, Snapshot{Achievements: &Achievements{DefinitionsComplete: false}}},
		{"incomplete chart", Query{Kind: "chart_mostplayed"}, Snapshot{Chart: &Chart{Type: "mostplayed", Complete: false, Entries: chart100(), SourceURL: "https://store.steampowered.com/charts/mostplayed"}}},
		{"market country mismatch", Query{Kind: "market", AppID: 10, Country: "CN"}, Snapshot{Market: &Market{Country: "US", Currency: "CNY", AmountMinor: &zero, MinorUnit: 2, Status: "quoted"}}},
		{"market unknown status", Query{Kind: "market", AppID: 10, Country: "CN"}, Snapshot{Market: &Market{Country: "CN", Status: "weird"}}},
		{"verified without evidence", Query{Kind: "market", AppID: 10, Country: "CN"}, Snapshot{Market: &Market{Country: "CN", Currency: "CNY", AmountMinor: &zero, MinorUnit: 2, Status: "quoted", MarketVerified: true}}},
		{"blended rate provider", Query{Kind: "fx"}, Snapshot{Rates: &Rates{Provider: "mix", Items: []Rate{{Base: "EUR", Quote: "CNY", Date: "2026-10-01", Decimal: "7.8"}}}}},
	}
	for _, c := range cases {
		if e := c.s.Validate(c.q); e == nil {
			t.Fatalf("%s accepted", c.name)
		}
	}
}

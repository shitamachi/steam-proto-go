package externalfacts

import (
	"encoding/json"
	"testing"
	"time"
)

func identity() Fact {
	return Fact{Version: 1, Kind: "external_identity", Provider: "itad", AppID: 10, Country: "JP", ExternalID: "01849783-6a26-7147-ab32-71804ca47e8e", MappingRevision: Revision("itad", "01849783-6a26-7147-ab32-71804ca47e8e", 61), ShopID: 61, Status: "present"}
}
func TestExactMoneyAndBoundaries(t *testing.T) {
	for _, x := range []struct {
		value, currency string
		want            int64
		valid           bool
	}{{"1480", "JPY", 1480, true}, {"1.48", "USD", 148, true}, {"0", "CNY", 0, true}, {"1.5", "KRW", 0, false}, {"0.001", "EUR", 0, false}, {"-1", "USD", 0, false}, {"92233720368547758.08", "USD", 0, false}, {"1", "UNKNOWN", 0, false}} {
		v, e := Minor(x.value, x.currency)
		if (e == nil) != x.valid || x.valid && v != x.want {
			t.Fatalf("%+v got %d %v", x, v, e)
		}
	}
	for _, c := range Countries {
		if !Market(c) {
			t.Fatal(c)
		}
	}
	if Market("ZZ") || Market("cn") {
		t.Fatal("invalid market accepted")
	}
}
func TestStableChunksReplayAndConflict(t *testing.T) {
	id := identity()
	now := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	p, r := int64(0), int64(1000)
	events := []Event{{At: now, Currency: "JPY", MinorUnit: 0, Price: &p, Regular: &r, Discount: 100, Status: "quoted"}, {At: now.Add(-time.Minute), Currency: "UNKNOWN", Status: "no_quote"}}
	chunks, e := Chunks(id, events)
	if e != nil || len(chunks) != 2 {
		t.Fatal(chunks, e)
	}
	for _, c := range chunks {
		b, _ := json.Marshal(c)
		if _, e = Decode(b); e != nil {
			t.Fatal(e)
		}
		merged, e := Merge(c, c)
		if e != nil || len(merged.Chunk.Events) != 1 {
			t.Fatal("replay duplicate", merged, e)
		}
		fresh := c
		fresh.FetchedAt = now
		old := c
		old.FetchedAt = now.Add(-time.Hour)
		merged, e = Merge(fresh, old)
		if e != nil || !merged.FetchedAt.Equal(fresh.FetchedAt) {
			t.Fatal("old replay changed freshness", merged, e)
		}
	}
	c := chunks[0]
	for _, v := range chunks {
		if v.Chunk.Currency == "JPY" {
			c = v
		}
	}
	conflict := c
	copyChunk := *c.Chunk
	copyChunk.Events = append([]Event{}, c.Chunk.Events...)
	v := int64(1)
	copyChunk.Events[0].Price = &v
	copyChunk.Events[0].Discount = 99
	conflict.Chunk = &copyChunk
	if _, e := Merge(c, conflict); e == nil {
		t.Fatal("conflict hidden")
	}
	wrong := c
	wrong.Country = "US"
	if _, e := Merge(c, wrong); e == nil {
		t.Fatal("cross-country merge")
	}
}
func TestFactRejectsShapeAndFuture(t *testing.T) {
	id := identity()
	if e := id.Validate(); e != nil {
		t.Fatal(e)
	}
	for _, edit := range []func(*Fact){func(v *Fact) { v.Kind = "external_duration" }, func(v *Fact) { v.Country = "ZZ" }, func(v *Fact) { v.FetchedAt = time.Now().Add(time.Hour) }, func(v *Fact) { v.Status = "missing"; v.Low = &Event{} }, func(v *Fact) { v.ExternalID = "broken" }} {
		v := id
		edit(&v)
		if v.Validate() == nil {
			t.Fatalf("accepted %+v", v)
		}
	}
	cov := id
	cov.Kind = "external_coverage"
	cov.Coverage = &Coverage{Since: time.Now().Add(-time.Hour), Until: time.Now(), ResponseComplete: true, WindowComplete: true}
	if cov.Validate() == nil {
		t.Fatal("unproven coverage accepted")
	}
	if _, e := Decode(make([]byte, MaxPayload+1)); e == nil {
		t.Fatal("unbounded JSON")
	}
}
func TestQueryScopesAndHistoryWindow(t *testing.T) {
	id := identity()
	valid := []Query{{Provider: "itad", Kind: "identity", AppID: 10, Country: "JP"}, {Provider: "itad", Kind: "history", AppID: 10, Country: "JP", ExternalID: id.ExternalID, ShopID: 61, Since: time.Now().Add(-time.Hour).Format(time.RFC3339)}, {Provider: "itad", Kind: "low", AppID: 10, Country: "JP", ExternalID: id.ExternalID, ShopID: 61}, {Provider: "igdb", Kind: "identity", AppID: 10}, {Provider: "igdb", Kind: "duration", AppID: 10, ExternalID: "2"}}
	for _, q := range valid {
		if e := q.Validate(); e != nil {
			t.Fatal(q, e)
		}
	}
	invalid := []Query{{Provider: "other", Kind: "identity", AppID: 10}, {Provider: "itad", Kind: "identity", AppID: 0, Country: "JP"}, {Provider: "itad", Kind: "identity", AppID: 10, Country: "ZZ"}, {Provider: "itad", Kind: "duration", AppID: 10, Country: "JP"}, {Provider: "itad", Kind: "low", AppID: 10, Country: "JP", ExternalID: "bad", ShopID: 61}, {Provider: "igdb", Kind: "duration", AppID: 10, ExternalID: "0"}, {Provider: "igdb", Kind: "identity", AppID: 10, Country: "US"}, {Provider: "igdb", Kind: "identity", AppID: 10, Since: "2026-01-01T00:00:00Z"}, {Provider: "itad", Kind: "history", AppID: 10, Country: "JP", ExternalID: id.ExternalID, ShopID: 61, Since: time.Now().AddDate(0, -25, 0).Format(time.RFC3339)}, {Provider: "itad", Kind: "history", AppID: 10, Country: "JP", ExternalID: id.ExternalID, ShopID: 61, Since: "bad"}, {Provider: "itad", Kind: "identity", AppID: 10, Country: "JP", Since: "2026-01-01T00:00:00Z"}}
	for _, q := range invalid {
		if q.Validate() == nil {
			t.Fatal("invalid query accepted", q)
		}
	}
	if !ProviderKind("external_low") || ProviderKind("news") {
		t.Fatal("provider kind")
	}
}
func TestCoverageDurationLowAndChunkShapeContracts(t *testing.T) {
	id := identity()
	now := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	money := int64(100)
	event := Event{At: now, Currency: "JPY", Price: &money, Regular: &money, Status: "quoted"}
	low := id
	low.Kind = "external_low"
	low.Low = &event
	if low.Validate() != nil {
		t.Fatal("valid low")
	}
	duration := Fact{Version: 1, Kind: "external_duration", Provider: "igdb", AppID: 10, ExternalID: "2", MappingRevision: Revision("igdb", "2", 0), Status: "present", Duration: &Duration{Normally: &money, UpdatedAt: now, Count: 3}}
	if duration.Validate() != nil {
		t.Fatal("valid duration")
	}
	cov := id
	cov.Kind = "external_coverage"
	cov.Coverage = &Coverage{Since: now.Add(-time.Hour), Until: now, Count: 0, ResponseComplete: true}
	if cov.Validate() != nil {
		t.Fatal("empty coverage")
	}
	chunk := id
	chunk.Kind = "external_history"
	chunk.Chunk = &Chunk{Period: now.Format("2006-01"), Shard: (now.Day() - 1) / 8, Currency: "JPY", Events: []Event{event}}
	if chunk.Validate() != nil {
		t.Fatal("valid chunk")
	}
	for _, status := range []string{"missing", "ambiguous", "revoked"} {
		missing := id
		missing.Status = status
		if missing.Validate() != nil {
			t.Fatal(status)
		}
	}
	cases := []Fact{low, duration, cov, chunk, id}
	mutations := [][]func(*Fact){{func(v *Fact) { v.Low = nil }, func(v *Fact) { e := Event{At: now, Currency: "UNKNOWN", Status: "no_quote"}; v.Low = &e }}, {func(v *Fact) { d := *v.Duration; d.Count = -1; v.Duration = &d }, func(v *Fact) { d := *v.Duration; d.MinimumSamples = 101; v.Duration = &d }, func(v *Fact) { d := *v.Duration; p := int64(-1); d.Normally = &p; v.Duration = &d }, func(v *Fact) { v.Duration = nil }}, {func(v *Fact) { c := *v.Coverage; c.Count = 1; v.Coverage = &c }, func(v *Fact) { c := *v.Coverage; c.Earliest = &now; v.Coverage = &c }, func(v *Fact) { v.Coverage = nil }}, {func(v *Fact) { c := *v.Chunk; c.Shard = 4; v.Chunk = &c }, func(v *Fact) { c := *v.Chunk; c.Period = "wrong"; v.Chunk = &c }, func(v *Fact) { c := *v.Chunk; c.Currency = "USD"; v.Chunk = &c }, func(v *Fact) { c := *v.Chunk; c.Events = append(c.Events, event); v.Chunk = &c }}, {func(v *Fact) { v.MappingRevision = "wrong" }, func(v *Fact) { v.Low = &event }, func(v *Fact) {
		v.Provider = "igdb"
		v.Country = ""
		v.ShopID = 0
		v.ExternalID = "2"
		v.MappingRevision = Revision("igdb", "2", 0)
		v.Kind = "external_low"
	}}}
	for i, tests := range mutations {
		for _, edit := range tests {
			v := cases[i]
			edit(&v)
			if v.Validate() == nil {
				t.Fatal("invalid contract accepted", v)
			}
		}
	}
	for _, bad := range []Event{{At: now, Currency: "UNKNOWN", Status: "no_quote", Price: &money}, {At: now, Currency: "JPY", Status: "quoted", Price: &money}, {At: now, Currency: "JPY", Status: "other"}, {At: now, Currency: "USD", MinorUnit: 0, Price: &money, Regular: &money, Status: "quoted"}} {
		if bad.Validate() == nil {
			t.Fatal("invalid event accepted", bad)
		}
	}
}
func TestMarketCurrencyFallbackIsRejected(t *testing.T) {
	currencies := []string{"CNY", "USD", "JPY", "GBP", "EUR", "EUR", "CAD", "AUD", "KRW", "BRL", "INR", "USD", "USD", "MXN", "PLN", "RUB", "TWD", "HKD", "SGD", "IDR", "THB", "VND", "MYR", "PHP", "NZD"}
	for i, country := range Countries {
		if !MarketCurrency(country, currencies[i]) {
			t.Fatal(country, currencies[i])
		}
	}
	if MarketCurrency("JP", "USD") || MarketCurrency("CN", "USD") || MarketCurrency("ZZ", "USD") {
		t.Fatal("market fallback")
	}
	id := identity()
	price := int64(100)
	event := Event{At: time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond), Currency: "USD", MinorUnit: 2, Price: &price, Regular: &price, Status: "quoted"}
	if _, e := Chunks(id, []Event{event}); e == nil {
		t.Fatal("US currency inserted in JP history")
	}
	id.Kind = "external_low"
	id.Low = &event
	if id.Validate() == nil {
		t.Fatal("recent fallback low accepted")
	}
	event.At = event.At.AddDate(-15, 0, 0)
	id.Low = &event
	if e := id.Validate(); e != nil {
		t.Fatal("legacy source low lost original currency", e)
	}
}

func TestCollectedRequiresFetchTimeButLegacyDecodeRemainsCompatible(t *testing.T) {
	legacy := identity()
	if err := legacy.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(legacy)
	if _, err := Decode(raw); err != nil {
		t.Fatalf("legacy decode broken: %v", err)
	}
	if legacy.ValidateCollected() == nil {
		t.Fatal("new producer accepted missing fetch time")
	}
	legacy.FetchedAt = time.Now().UTC()
	if err := legacy.ValidateCollected(); err != nil {
		t.Fatal(err)
	}
}

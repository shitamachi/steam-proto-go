package collectionfacts

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestExactMoney(t *testing.T) {
	for _, v := range []struct {
		amount       int64
		from, to     int32
		fr, tr, want string
	}{
		{1480, 0, 2, "160", "7.8", "7215"}, {0, 2, 2, "1", "7.8", "0"}, {1, 0, 0, "2", "1", "1"}, {999, 2, 0, "1", "160", "1598"}, {9223372036854775807, 2, 2, "1", "1", "9223372036854775807"},
	} {
		got, e := ConvertMinor(v.amount, v.from, v.to, v.fr, v.tr)
		if e != nil || got != v.want {
			t.Fatalf("%+v => %s %v", v, got, e)
		}
	}
	for _, v := range []struct {
		a int64
		u int32
		r string
	}{{-1, 2, "1"}, {1, 4, "1"}, {1, 2, "0"}, {1, 2, "NaN"}, {1, -1, "1"}} {
		if _, e := ConvertMinor(v.a, v.u, 2, v.r, "1"); e == nil {
			t.Fatal(v)
		}
	}
	for _, c := range []string{"JPY", "KRW", "CNY", "USD", "GBP", "EUR"} {
		if _, ok := MinorUnit(c); !ok {
			t.Fatal(c)
		}
	}
	if _, ok := MinorUnit("???"); ok {
		t.Fatal("unknown currency")
	}
}
func TestQueriesAndURLScopes(t *testing.T) {
	for _, q := range []Query{{Kind: "players", AppID: 10}, {Kind: "news", AppID: 10, Before: 1}, {Kind: "achievements", AppID: 10, Language: "schinese"}, {Kind: "controller", AppID: 10}, {Kind: "market", AppID: 10, Country: "JP"}, {Kind: "chart_topselling", Country: "CN"}, {Kind: "chart_mostplayed"}, {Kind: "fx"}} {
		if e := q.Validate(); e != nil {
			t.Fatal(e)
		}
		if q.Scope() == "" {
			t.Fatal(q)
		}
	}
	for _, q := range []Query{{Kind: "unknown"}, {Kind: "players"}, {Kind: "players", AppID: 1 << 32}, {Kind: "fx", AppID: 1}, {Kind: "market", AppID: 10, Country: "FR"}, {Kind: "chart_topselling", Country: "US"}, {Kind: "achievements", AppID: 10, Language: "german"}, {Kind: "news", AppID: 10, Language: "english"}, {Kind: "players", AppID: 10, Country: "CN"}, {Kind: "players", AppID: 10, Before: 1}, {Kind: "news", AppID: 10, Before: -1}} {
		if q.Validate() == nil {
			t.Fatal(q)
		}
	}
	for _, u := range []string{"http://store.steampowered.com/a", "https://user:pass@store.steampowered.com/a", "javascript:alert(1)", "https://store.steampowered.com.evil/a"} {
		if SteamURL(u) {
			t.Fatal(u)
		}
	}
	if !SafeURL("https://example.com/a") || !SteamURL("https://steamcommunity.com/a") {
		t.Fatal("valid URL")
	}
	if AppString(10) != "10" {
		t.Fatal("identity")
	}
}
func TestSnapshotBoundaries(t *testing.T) {
	zero := 0.0
	amount := int64(0)
	chart := &Chart{Type: "mostplayed", Complete: true, SourceURL: "https://store.steampowered.com/charts/mostplayed"}
	for i := 1; i <= 100; i++ {
		chart.Entries = append(chart.Entries, ChartEntry{Rank: int32(i), Type: "app", ID: int64(i), Title: "Game", URL: fmt.Sprintf("https://store.steampowered.com/app/%d/", i)})
	}
	good := []struct {
		q Query
		s Snapshot
	}{
		{Query{Kind: "players", AppID: 10}, Snapshot{Player: &Player{Count: 0}}},
		{Query{Kind: "controller", AppID: 10}, Snapshot{Controller: &Controller{Support: "unknown"}}},
		{Query{Kind: "news", AppID: 10}, Snapshot{News: &NewsPage{Items: []News{{ID: "1", Title: "Title", Summary: "text", URL: "https://steamcommunity.com/a", Classification: "official", PublishedAt: 1}}}}},
		{Query{Kind: "achievements", AppID: 10, Language: "english"}, Snapshot{Achievements: &Achievements{DefinitionsComplete: true, Items: []Achievement{{Name: "ZERO", Percent: &zero}}}}},
		{Query{Kind: "chart_mostplayed"}, Snapshot{Chart: chart}},
		{Query{Kind: "market", AppID: 10, Country: "CN"}, Snapshot{Market: &Market{Country: "CN", Currency: "CNY", AmountMinor: &amount, MinorUnit: 2, Status: "quoted", MarketVerified: true, EvidenceURL: "https://store.steampowered.com/app/10/?cc=cn"}}},
		{Query{Kind: "market", AppID: 10, Country: "CN"}, Snapshot{Market: &Market{Country: "CN", Status: "no_quote"}}},
		{Query{Kind: "fx"}, Snapshot{Rates: &Rates{Provider: "ecb", Items: []Rate{{Base: "EUR", Quote: "CNY", Date: "2026-10-01", Decimal: "7.8"}}}}},
	}
	for _, v := range good {
		if e := v.s.Validate(v.q); e != nil {
			t.Fatal(e)
		}
		b, _ := json.Marshal(v.s)
		if _, e := Decode(v.q, b); e != nil {
			t.Fatal(e)
		}
	}
	for _, b := range [][]byte{nil, []byte("{"), []byte(strings.Repeat("x", MaxPayload+1))} {
		if _, e := Decode(good[0].q, b); e == nil {
			t.Fatal("invalid JSON")
		}
	}
	for _, mutate := range []func(){func() { good[0].s.Player.Count = -1 }, func() { good[1].s.Controller.Support = "unsupported" }, func() { good[2].s.News.Items[0].URL = "http://example.com" }, func() { good[3].s.Achievements.Items[0].Name = "" }, func() { chart.Entries[0].Rank = 2 }, func() { good[5].s.Market.MinorUnit = 0 }, func() { good[6].s.Market.Currency = "CNY" }, func() { good[7].s.Rates.Items[0].Decimal = "0" }} {
		mutate()
	}
	for _, v := range good {
		if v.s.Validate(v.q) == nil {
			t.Fatal("invalid domain accepted", v.q.Kind)
		}
	}
	nan := math.NaN()
	if (Snapshot{Achievements: &Achievements{DefinitionsComplete: true, Items: []Achievement{{Name: "nan", Percent: &nan}}}}).Validate(Query{Kind: "achievements", AppID: 10, Language: "english"}) == nil {
		t.Fatal("NaN")
	}
	if (Snapshot{}).Validate(good[0].q) == nil || (Snapshot{Player: &Player{}, News: &NewsPage{}}).Validate(good[0].q) == nil {
		t.Fatal("oneof")
	}
}

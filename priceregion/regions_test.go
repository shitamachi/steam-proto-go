package priceregion

import (
	"strings"
	"testing"
)

// Expected identities are deliberately independent of the registry under test.
func TestAll41PricingRegions(t *testing.T) {
	expected := map[string]string{"cn": "CNY", "us": "USD", "gb": "GBP", "de": "EUR", "ru": "RUB", "br": "BRL", "jp": "JPY", "id": "IDR", "my": "MYR", "ph": "PHP", "sg": "SGD", "th": "THB", "vn": "VND", "kr": "KRW", "ua": "UAH", "mx": "MXN", "ca": "CAD", "au": "AUD", "nz": "NZD", "no": "NOK", "pl": "PLN", "ch": "CHF", "in": "INR", "cl": "CLP", "pe": "PEN", "co": "COP", "za": "ZAR", "hk": "HKD", "tw": "TWD", "sa": "SAR", "ae": "AED", "il": "ILS", "kz": "KZT", "kw": "KWD", "qa": "QAR", "cr": "CRC", "uy": "UYU", "az": "USD-CIS", "ar": "USD-LATAM", "tr": "USD-MENA", "pk": "USD-SASIA"}
	all := All()
	if len(all) != 41 {
		t.Fatalf("regions=%d", len(all))
	}
	currencies := map[string]bool{}
	seen := map[string]bool{}
	usd := 0
	for _, r := range all {
		if expected[r.Country] != r.ID || seen[r.ID] {
			t.Fatalf("bad or duplicated region %+v", r)
		}
		currency := expected[r.Country]
		if strings.HasPrefix(currency, "USD-") {
			currency = "USD"
		}
		if r.Currency != currency {
			t.Fatalf("wrong currency for %+v: want %s", r, currency)
		}
		seen[r.ID] = true
		currencies[r.Currency] = true
		if r.Currency == "USD" {
			usd++
		}
		got, ok := ByCountry(r.Country)
		if !ok || got != r {
			t.Fatalf("lookup mismatch %+v", r)
		}
	}
	if len(currencies) != 37 || usd != 5 {
		t.Fatalf("currencies=%d USD=%d", len(currencies), usd)
	}
	all[0].Country = "xx"
	if All()[0].Country == "xx" {
		t.Fatal("registry is mutable")
	}
}

func TestPriceCountryConfiguration(t *testing.T) {
	all, err := ParseCountries("all")
	if err != nil || len(all) != 41 {
		t.Fatalf("all: %d %v", len(all), err)
	}
	for _, s := range []string{"cn,cn", "xx", "fr", "all,cn", "usd", "us,US", "cn,,us"} {
		if _, err := ParseCountries(s); err == nil {
			t.Errorf("accepted ambiguous/nonrepresentative config %q", s)
		}
	}
	r, err := ParseCountries("CN,us,jp,az,ar,tr,pk")
	if err != nil || len(r) != 7 {
		t.Fatalf("explicit regions: %v", err)
	}
	for _, cc := range []string{"fr", "CN", "usd", ""} {
		if _, ok := ByCountry(cc); ok {
			t.Errorf("unexpected request country %q", cc)
		}
	}
}

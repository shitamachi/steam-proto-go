// Package priceregion defines Steam pricing identities, not ISO currency scales
// or every country where Steam sells games. This is a handwritten data contract.
package priceregion

import (
	"fmt"
	"strings"
)

const Version = "steam-41-v1"

// Country is the actual representative country sent as the Store API cc value.
// ID stays distinct even when multiple pricing regions use the same currency.
type Region struct{ ID, Currency, Country string }

// Sources (verified 2026-10-23):
// https://partner.steamgames.com/pricing/explorer
// https://partner.steamgames.com/doc/store/pricing/currencies
var regions = [...]Region{
	{"CNY", "CNY", "cn"}, {"USD", "USD", "us"}, {"GBP", "GBP", "gb"}, {"EUR", "EUR", "de"},
	{"RUB", "RUB", "ru"}, {"BRL", "BRL", "br"}, {"JPY", "JPY", "jp"}, {"IDR", "IDR", "id"},
	{"MYR", "MYR", "my"}, {"PHP", "PHP", "ph"}, {"SGD", "SGD", "sg"}, {"THB", "THB", "th"},
	{"VND", "VND", "vn"}, {"KRW", "KRW", "kr"}, {"UAH", "UAH", "ua"}, {"MXN", "MXN", "mx"},
	{"CAD", "CAD", "ca"}, {"AUD", "AUD", "au"}, {"NZD", "NZD", "nz"}, {"NOK", "NOK", "no"},
	{"PLN", "PLN", "pl"}, {"CHF", "CHF", "ch"}, {"INR", "INR", "in"}, {"CLP", "CLP", "cl"},
	{"PEN", "PEN", "pe"}, {"COP", "COP", "co"}, {"ZAR", "ZAR", "za"}, {"HKD", "HKD", "hk"},
	{"TWD", "TWD", "tw"}, {"SAR", "SAR", "sa"}, {"AED", "AED", "ae"}, {"ILS", "ILS", "il"},
	{"KZT", "KZT", "kz"}, {"KWD", "KWD", "kw"}, {"QAR", "QAR", "qa"}, {"CRC", "CRC", "cr"},
	{"UYU", "UYU", "uy"}, {"USD-CIS", "USD", "az"}, {"USD-LATAM", "USD", "ar"},
	{"USD-MENA", "USD", "tr"}, {"USD-SASIA", "USD", "pk"},
}

func All() []Region { return append([]Region(nil), regions[:]...) }
func ByCountry(country string) (Region, bool) {
	for _, r := range regions {
		if r.Country == country {
			return r, true
		}
	}
	return Region{}, false
}

// ParseCountries deliberately does not reuse an ISO-country "all" expansion.
func ParseCountries(raw string) ([]Region, error) {
	if strings.EqualFold(strings.TrimSpace(raw), "all") {
		return All(), nil
	}
	out := []Region{}
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		cc := strings.ToLower(strings.TrimSpace(part))
		r, ok := ByCountry(cc)
		if !ok || seen[cc] {
			return nil, fmt.Errorf("unknown or duplicate Steam pricing representative %q", part)
		}
		seen[cc] = true
		out = append(out, r)
	}
	return out, nil
}

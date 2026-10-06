// Package externalfacts defines bounded P6 facts without transport or credentials.
package externalfacts

import (
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const MaxPayload = 128 << 10
const MaxFacts = 512
const MaxResponse = 4 << 20

var Countries = []string{"CN", "US", "JP", "GB", "DE", "FR", "CA", "AU", "KR", "BR", "IN", "TR", "AR", "MX", "PL", "RU", "TW", "HK", "SG", "ID", "TH", "VN", "MY", "PH", "NZ"}
var uuid = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
var moneyText = regexp.MustCompile(`^[0-9]{1,20}(?:\.[0-9]{1,6})?$`)
var numeric = regexp.MustCompile(`^[1-9][0-9]{0,15}$`)

func Market(c string) bool {
	for _, v := range Countries {
		if c == v {
			return true
		}
	}
	return false
}

// MarketCurrency rejects a likely provider fallback in the recent history
// window. Legacy Turkish/Argentinian amounts remain distinct from USD.
func MarketCurrency(country, currency string) bool {
	switch country {
	case "CN":
		return currency == "CNY"
	case "US":
		return currency == "USD"
	case "JP":
		return currency == "JPY"
	case "GB":
		return currency == "GBP"
	case "DE", "FR":
		return currency == "EUR"
	case "CA":
		return currency == "CAD"
	case "AU":
		return currency == "AUD"
	case "KR":
		return currency == "KRW"
	case "BR":
		return currency == "BRL"
	case "IN":
		return currency == "INR"
	case "TR":
		return currency == "USD" || currency == "TRY"
	case "AR":
		return currency == "USD" || currency == "ARS"
	case "MX":
		return currency == "MXN"
	case "PL":
		return currency == "PLN"
	case "RU":
		return currency == "RUB"
	case "TW":
		return currency == "TWD"
	case "HK":
		return currency == "HKD"
	case "SG":
		return currency == "SGD"
	case "ID":
		return currency == "IDR"
	case "TH":
		return currency == "THB"
	case "VN":
		return currency == "VND"
	case "MY":
		return currency == "MYR"
	case "PH":
		return currency == "PHP"
	case "NZ":
		return currency == "NZD"
	}
	return false
}
func Unit(c string) (int, bool) {
	switch c {
	case "JPY", "KRW", "VND":
		return 0, true
	case "CNY", "USD", "GBP", "EUR", "CAD", "AUD", "BRL", "INR", "TRY", "ARS", "MXN", "PLN", "RUB", "TWD", "HKD", "SGD", "IDR", "THB", "MYR", "PHP", "NZD":
		return 2, true
	}
	return 0, false
}
func Minor(decimal, currency string) (int64, error) {
	u, ok := Unit(currency)
	if len(decimal) > 32 || !moneyText.MatchString(decimal) {
		return 0, fmt.Errorf("invalid money text")
	}
	r, valid := new(big.Rat).SetString(decimal)
	if !ok || !valid || r.Sign() < 0 {
		return 0, fmt.Errorf("invalid money")
	}
	r.Mul(r, new(big.Rat).SetInt64(int64Pow10(u)))
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, fmt.Errorf("fractional or overflowing money")
	}
	return r.Num().Int64(), nil
}
func int64Pow10(n int) int64 {
	v := int64(1)
	for i := 0; i < n; i++ {
		v *= 10
	}
	return v
}

type Query struct {
	Provider   string `json:"provider"`
	Kind       string `json:"kind"`
	AppID      int64  `json:"appId"`
	Country    string `json:"country,omitempty"`
	ExternalID string `json:"externalId,omitempty"`
	Since      string `json:"since,omitempty"`
	ShopID     int64  `json:"shopId,omitempty"`
}

func (q Query) Validate() error {
	if q.AppID <= 0 || q.AppID > 2147483647 {
		return fmt.Errorf("invalid AppID")
	}
	switch q.Provider {
	case "itad":
		if !Market(q.Country) {
			return fmt.Errorf("invalid country")
		}
		if q.Kind != "identity" && q.Kind != "history" && q.Kind != "low" {
			return fmt.Errorf("invalid ITAD operation")
		}
		if q.Kind != "identity" && (!uuid.MatchString(q.ExternalID) || q.ShopID <= 0) {
			return fmt.Errorf("invalid ITAD identity")
		}
	case "igdb":
		if q.Country != "" || q.ShopID != 0 || q.Since != "" || (q.Kind != "identity" && q.Kind != "duration") {
			return fmt.Errorf("invalid IGDB operation")
		}
		if q.Kind == "duration" && !numeric.MatchString(q.ExternalID) {
			return fmt.Errorf("invalid IGDB identity")
		}
	default:
		return fmt.Errorf("invalid provider")
	}
	if q.Kind == "history" {
		t, e := time.Parse(time.RFC3339, q.Since)
		if e != nil || t.After(time.Now().Add(time.Minute)) || t.Before(time.Now().AddDate(0, -24, -1)) {
			return fmt.Errorf("invalid history boundary")
		}
	} else if q.Since != "" {
		return fmt.Errorf("unexpected boundary")
	}
	return nil
}

type Event struct {
	At        time.Time `json:"at"`
	Currency  string    `json:"currency"`
	MinorUnit int       `json:"minorUnit"`
	Regular   *int64    `json:"regular,omitempty"`
	Price     *int64    `json:"price,omitempty"`
	Discount  int       `json:"discount"`
	Status    string    `json:"status"`
}

func (e Event) Validate() error {
	u, ok := Unit(e.Currency)
	if (!ok && !(e.Status == "no_quote" && e.Currency == "UNKNOWN")) || e.MinorUnit != u || e.At.IsZero() || e.At.After(time.Now().Add(time.Minute)) || e.Discount < 0 || e.Discount > 100 {
		return fmt.Errorf("invalid price event")
	}
	if e.Status == "no_quote" {
		if e.Price != nil || e.Regular != nil || e.Discount != 0 {
			return fmt.Errorf("invalid missing quote")
		}
		return nil
	}
	if e.Status != "quoted" || e.Price == nil || e.Regular == nil || *e.Price < 0 || *e.Regular < 0 || *e.Price > *e.Regular {
		return fmt.Errorf("invalid quote")
	}
	return nil
}

type Chunk struct {
	Period   string  `json:"period"`
	Shard    int     `json:"shard"`
	Currency string  `json:"currency"`
	Events   []Event `json:"events"`
}
type Coverage struct {
	Since            time.Time  `json:"since"`
	Until            time.Time  `json:"until"`
	Earliest         *time.Time `json:"earliest,omitempty"`
	Latest           *time.Time `json:"latest,omitempty"`
	Count            int        `json:"count"`
	ResponseComplete bool       `json:"responseComplete"`
	WindowComplete   bool       `json:"windowComplete"`
}
type Duration struct {
	Hastily        *int64    `json:"hastily,omitempty"`
	Normally       *int64    `json:"normally,omitempty"`
	Completely     *int64    `json:"completely,omitempty"`
	Count          int64     `json:"count"`
	UpdatedAt      time.Time `json:"updatedAt"`
	MinimumSamples int64     `json:"minimumSamples,omitempty"`
}
type Fact struct {
	Version         int       `json:"version"`
	FetchedAt       time.Time `json:"fetchedAt,omitempty"`
	Kind            string    `json:"kind"`
	Provider        string    `json:"provider"`
	AppID           int64     `json:"appId"`
	Country         string    `json:"country,omitempty"`
	ExternalID      string    `json:"externalId,omitempty"`
	MappingRevision string    `json:"mappingRevision,omitempty"`
	ShopID          int64     `json:"shopId,omitempty"`
	Status          string    `json:"status"`
	Chunk           *Chunk    `json:"chunk,omitempty"`
	Coverage        *Coverage `json:"coverage,omitempty"`
	Low             *Event    `json:"low,omitempty"`
	Duration        *Duration `json:"duration,omitempty"`
}

func (f Fact) Scope() string {
	base := fmt.Sprintf("%s:%s:%d:%s", f.Kind, f.Provider, f.AppID, f.Country)
	if f.Kind != "external_identity" {
		base += ":" + f.MappingRevision
	}
	if f.Chunk != nil {
		base += fmt.Sprintf(":%s:%s:%d", f.Chunk.Currency, f.Chunk.Period, f.Chunk.Shard)
	}
	return base
}
func (f Fact) Validate() error {
	if f.FetchedAt.After(time.Now().Add(time.Minute)) || len(f.ExternalID) > 64 || f.Version != 1 || f.AppID <= 0 || f.AppID > 2147483647 || (f.Provider != "itad" && f.Provider != "igdb") || len(f.MappingRevision) > 80 {
		return fmt.Errorf("invalid fact identity")
	}
	if f.Provider == "itad" && !Market(f.Country) || f.Provider == "igdb" && f.Country != "" {
		return fmt.Errorf("invalid fact market")
	}
	if f.Status != "present" && f.Status != "missing" && f.Status != "ambiguous" && f.Status != "revoked" {
		return fmt.Errorf("invalid fact status")
	}
	if f.Status == "present" {
		if f.Provider == "itad" && (!uuid.MatchString(f.ExternalID) || f.ShopID <= 0) || f.Provider == "igdb" && !numeric.MatchString(f.ExternalID) {
			return fmt.Errorf("invalid external identity")
		}
		if f.MappingRevision != Revision(f.Provider, f.ExternalID, f.ShopID) {
			return fmt.Errorf("missing mapping revision")
		}
	}
	n := 0
	for _, v := range []bool{f.Chunk != nil, f.Coverage != nil, f.Low != nil, f.Duration != nil} {
		if v {
			n++
		}
	}
	if f.Provider == "igdb" && f.ShopID != 0 || f.Provider == "igdb" && (f.Kind != "external_identity" && f.Kind != "external_duration") || f.Provider == "itad" && f.Kind == "external_duration" {
		return fmt.Errorf("provider kind mismatch")
	}
	if f.Status != "present" {
		if f.Kind != "external_identity" && f.Kind != "external_duration" && f.Kind != "external_low" || n != 0 {
			return fmt.Errorf("invalid missing fact")
		}
		return nil
	}
	switch f.Kind {
	case "external_identity":
		if n != 0 {
			return fmt.Errorf("invalid identity content")
		}
	case "external_history":
		if f.Provider != "itad" || n != 1 || f.Chunk == nil || len(f.Chunk.Events) < 1 || len(f.Chunk.Events) > 256 || f.Chunk.Shard < 0 || f.Chunk.Shard > 3 {
			return fmt.Errorf("invalid history chunk")
		}
		if _, e := time.Parse("2006-01", f.Chunk.Period); e != nil {
			return e
		}
		last := time.Time{}
		for _, e := range f.Chunk.Events {
			if e.Validate() != nil || (e.Status == "quoted" && !MarketCurrency(f.Country, e.Currency)) || e.Currency != f.Chunk.Currency || e.At.UTC().Format("2006-01") != f.Chunk.Period || (e.At.UTC().Day()-1)/8 != f.Chunk.Shard || !e.At.After(last) {
				return fmt.Errorf("invalid chunk event")
			}
			last = e.At
		}
	case "external_coverage":
		if f.Provider != "itad" || n != 1 || f.Coverage == nil {
			return fmt.Errorf("invalid coverage")
		}
		c := f.Coverage
		if c.Since.IsZero() || c.Until.After(time.Now().Add(time.Minute)) || c.Until.Before(c.Since) || c.Count < 0 || c.Count > 10000 || !c.ResponseComplete || c.WindowComplete {
			return fmt.Errorf("unproven coverage")
		}
		if c.Count == 0 && (c.Earliest != nil || c.Latest != nil) || c.Count > 0 && (c.Earliest == nil || c.Latest == nil || c.Latest.Before(*c.Earliest) || c.Earliest.Before(c.Since) || c.Latest.After(c.Until)) {
			return fmt.Errorf("invalid coverage bounds")
		}
	case "external_low":
		if f.Provider != "itad" || n != 1 || f.Low == nil || f.Low.Validate() != nil || f.Low.Status != "quoted" {
			return fmt.Errorf("invalid low")
		}
		if f.Low.At.After(time.Now().AddDate(0, -24, 0)) && !MarketCurrency(f.Country, f.Low.Currency) {
			return fmt.Errorf("low market currency mismatch")
		}
	case "external_duration":
		if f.Provider != "igdb" || n != 1 || f.Duration == nil {
			return fmt.Errorf("invalid duration")
		}
		d := f.Duration
		if d.MinimumSamples < 0 || d.MinimumSamples > 100 || d.UpdatedAt.IsZero() || d.Count < 0 || d.Count > 10000000 || d.UpdatedAt.After(time.Now().Add(time.Minute)) {
			return fmt.Errorf("invalid samples")
		}
		for _, v := range []*int64{d.Hastily, d.Normally, d.Completely} {
			if v != nil && (*v < 0 || *v > 10*365*86400) {
				return fmt.Errorf("invalid duration value")
			}
		}
	default:
		return fmt.Errorf("invalid fact kind")
	}
	b, e := json.Marshal(f)
	if e != nil || len(b) > MaxPayload {
		return fmt.Errorf("oversized fact")
	}
	return nil
}

// ValidateCollected applies the producer contract. Validate/Decode remain
// compatible with legacy v1 facts lacking supplier fetch time.
func (f Fact) ValidateCollected() error {
	if f.FetchedAt.IsZero() {
		return fmt.Errorf("missing source fetch timestamp")
	}
	return f.Validate()
}

func Decode(raw []byte) (Fact, error) {
	var f Fact
	if len(raw) > MaxPayload || json.Unmarshal(raw, &f) != nil {
		return f, fmt.Errorf("invalid external fact JSON")
	}
	return f, f.Validate()
}
func Revision(provider, id string, shop int64) string {
	return provider + "-" + id + "-" + strconv.FormatInt(shop, 10)
}
func Chunks(identity Fact, events []Event) ([]Fact, error) {
	by := map[string]*Chunk{}
	for _, e := range events {
		e.At = e.At.UTC().Truncate(time.Microsecond)
		if err := e.Validate(); err != nil {
			return nil, err
		}
		period := e.At.UTC().Format("2006-01")
		shard := (e.At.UTC().Day() - 1) / 8
		k := fmt.Sprintf("%s:%s:%d", e.Currency, period, shard)
		c := by[k]
		if c == nil {
			c = &Chunk{Period: period, Shard: shard, Currency: e.Currency}
			by[k] = c
		}
		c.Events = append(c.Events, e)
	}
	keys := []string{}
	for k := range by {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := []Fact{}
	for _, k := range keys {
		c := by[k]
		sort.Slice(c.Events, func(i, j int) bool { return c.Events[i].At.Before(c.Events[j].At) })
		dedup := []Event{}
		for _, v := range c.Events {
			if len(dedup) > 0 && dedup[len(dedup)-1].At.Equal(v.At) {
				a, _ := json.Marshal(dedup[len(dedup)-1])
				b, _ := json.Marshal(v)
				if string(a) != string(b) {
					return nil, fmt.Errorf("same-time conflict")
				}
				continue
			}
			dedup = append(dedup, v)
		}
		c.Events = dedup
		f := identity
		f.Kind = "external_history"
		f.Chunk = c
		if e := f.Validate(); e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, nil
}
func Merge(prior, incoming Fact) (Fact, error) {
	if prior.Scope() != incoming.Scope() || prior.Chunk == nil || incoming.Chunk == nil {
		return incoming, fmt.Errorf("chunk identity conflict")
	}
	events := append(append([]Event{}, prior.Chunk.Events...), incoming.Chunk.Events...)
	base := incoming
	if prior.FetchedAt.After(base.FetchedAt) {
		base.FetchedAt = prior.FetchedAt
	}
	base.Chunk = nil
	fs, e := Chunks(base, events)
	if e != nil || len(fs) != 1 {
		if e == nil {
			e = fmt.Errorf("chunk split conflict")
		}
		return incoming, e
	}
	return fs[0], nil
}
func ProviderKind(kind string) bool { return strings.HasPrefix(kind, "external_") }

// Package collectionfacts is the bounded, credential-free P5 source contract.
// It contains no transport, database clients or generated RPC types.
package collectionfacts

import (
	"encoding/json"
	"fmt"
	"math/big"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const MaxPayload = 512 << 10

type Query struct {
	Kind     string `json:"kind"`
	AppID    int64  `json:"appId"`
	Country  string `json:"country"`
	Language string `json:"language"`
	Before   int64  `json:"before,omitempty"`
}

func (q Query) Validate() error {
	switch q.Kind {
	case "players", "news", "achievements", "controller", "market":
		if q.AppID <= 0 || q.AppID > 2147483647 {
			return fmt.Errorf("invalid AppID")
		}
	case "chart_mostplayed", "chart_topselling", "fx":
		if q.AppID != 0 {
			return fmt.Errorf("unexpected AppID")
		}
	default:
		return fmt.Errorf("unsupported fact kind")
	}
	if q.Kind == "market" {
		if !ValidMarket(q.Country) || q.Language != "" {
			return fmt.Errorf("unsupported market")
		}
	} else if q.Kind == "chart_topselling" {
		if q.Country != "CN" || q.Language != "" {
			return fmt.Errorf("unsupported chart market")
		}
	} else if q.Country != "" {
		return fmt.Errorf("unexpected country")
	}
	if q.Kind == "achievements" {
		if q.Language != "schinese" && q.Language != "english" {
			return fmt.Errorf("unsupported language")
		}
	} else if q.Language != "" {
		return fmt.Errorf("unexpected language")
	}
	if q.Before < 0 || (q.Before != 0 && q.Kind != "news") {
		return fmt.Errorf("invalid news boundary")
	}
	return nil
}
func ValidMarket(v string) bool { return v == "CN" || v == "US" || v == "JP" || v == "GB" || v == "DE" }
func (q Query) Scope() string {
	return fmt.Sprintf("%s:%d:%s:%s", q.Kind, q.AppID, q.Country, q.Language)
}

type Controller struct {
	Support string `json:"support"` // full, partial or unknown; absence is never unsupported.
}
type Player struct {
	Count int64 `json:"count"`
}
type News struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Summary        string `json:"summary"`
	URL            string `json:"url"`
	Feed           string `json:"feed"`
	Classification string `json:"classification"`
	PublishedAt    int64  `json:"publishedAt"`
}
type NewsPage struct {
	Items      []News `json:"items"`
	NextBefore int64  `json:"nextBefore"`
	Truncated  bool   `json:"truncated"`
}
type Achievement struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"displayName"`
	Description string   `json:"description"`
	Hidden      bool     `json:"hidden"`
	Icon        string   `json:"icon"`
	Percent     *float64 `json:"percent,omitempty"`
}
type Achievements struct {
	Items                []Achievement `json:"items"`
	DefinitionsComplete  bool          `json:"definitionsComplete"`
	PercentagesAvailable bool          `json:"percentagesAvailable"`
}
type ChartEntry struct {
	Rank  int32  `json:"rank"`
	Type  string `json:"type"`
	ID    int64  `json:"id"`
	Title string `json:"title"`
	URL   string `json:"url"`
}
type Chart struct {
	Type      string       `json:"type"`
	Country   string       `json:"country"`
	Complete  bool         `json:"complete"`
	Entries   []ChartEntry `json:"entries"`
	SourceURL string       `json:"sourceUrl"`
}
type Market struct {
	Country        string `json:"country"`
	Currency       string `json:"currency"`
	AmountMinor    *int64 `json:"amountMinor,omitempty"`
	MinorUnit      int32  `json:"minorUnit"`
	Status         string `json:"status"`
	MarketVerified bool   `json:"marketVerified"`
	EvidenceURL    string `json:"evidenceUrl,omitempty"`
}
type Rate struct {
	Base    string `json:"base"`
	Quote   string `json:"quote"`
	Date    string `json:"date"`
	Decimal string `json:"decimal"`
}
type Rates struct {
	Provider string `json:"provider"`
	Items    []Rate `json:"items"`
}
type Snapshot struct {
	Controller   *Controller   `json:"controller,omitempty"`
	Player       *Player       `json:"player,omitempty"`
	News         *NewsPage     `json:"news,omitempty"`
	Achievements *Achievements `json:"achievements,omitempty"`
	Chart        *Chart        `json:"chart,omitempty"`
	Market       *Market       `json:"market,omitempty"`
	Rates        *Rates        `json:"rates,omitempty"`
}

var currency = regexp.MustCompile(`^[A-Z]{3}$`)
var decimal = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,9})(?:\.[0-9]{1,12})?$`)
var digits = regexp.MustCompile(`^[0-9]{1,30}$`)

func SafeURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil && len(s) <= 2048
}
func SteamURL(s string) bool {
	u, e := url.Parse(s)
	return e == nil && SafeURL(s) && (u.Hostname() == "store.steampowered.com" || u.Hostname() == "steamcommunity.com")
}
func (s Snapshot) Validate(q Query) error {
	if e := q.Validate(); e != nil {
		return e
	}
	n := 0
	for _, v := range []bool{s.Player != nil, s.News != nil, s.Achievements != nil, s.Chart != nil, s.Market != nil, s.Rates != nil, s.Controller != nil} {
		if v {
			n++
		}
	}
	if n != 1 {
		return fmt.Errorf("expected one fact domain")
	}
	switch q.Kind {
	case "controller":
		if s.Controller == nil || (s.Controller.Support != "full" && s.Controller.Support != "partial" && s.Controller.Support != "unknown") {
			return fmt.Errorf("invalid controller fact")
		}
	case "players":
		if s.Player == nil || s.Player.Count < 0 {
			return fmt.Errorf("invalid player observation")
		}
	case "news":
		if s.News == nil || len(s.News.Items) > 100 || s.News.NextBefore < 0 {
			return fmt.Errorf("invalid news page")
		}
		ids := map[string]bool{}
		for _, v := range s.News.Items {
			if !digits.MatchString(v.ID) || ids[v.ID] || len(v.Title) == 0 || len(v.Title) > 1000 || len(v.Summary) > 4000 || !SafeURL(v.URL) || v.PublishedAt <= 0 || len(v.Feed) > 100 || (v.Classification != "official" && v.Classification != "external" && v.Classification != "unknown") {
				return fmt.Errorf("invalid news item")
			}
			ids[v.ID] = true
		}
	case "achievements":
		if s.Achievements == nil || !s.Achievements.DefinitionsComplete || len(s.Achievements.Items) > 1000 {
			return fmt.Errorf("invalid achievement definitions")
		}
		ids := map[string]bool{}
		for _, v := range s.Achievements.Items {
			if v.Name == "" || len(v.Name) > 200 || ids[v.Name] || len(v.DisplayName) > 1000 || len(v.Description) > 4000 || (v.Icon != "" && !SafeURL(v.Icon)) || (v.Percent != nil && (*v.Percent < 0 || *v.Percent > 100)) {
				return fmt.Errorf("invalid achievement")
			}
			ids[v.Name] = true
		}
	case "chart_mostplayed", "chart_topselling":
		if s.Chart == nil || s.Chart.Type != strings.TrimPrefix(q.Kind, "chart_") || s.Chart.Country != q.Country || !s.Chart.Complete || len(s.Chart.Entries) != 100 || !SteamURL(s.Chart.SourceURL) {
			return fmt.Errorf("incomplete official chart")
		}
		ids := map[string]bool{}
		for i, v := range s.Chart.Entries {
			key := fmt.Sprintf("%s:%d", v.Type, v.ID)
			if v.Rank != int32(i+1) || v.ID <= 0 || ids[key] || (v.Type != "app" && v.Type != "package" && v.Type != "bundle") || v.Title == "" || len(v.Title) > 1000 || !SteamURL(v.URL) {
				return fmt.Errorf("invalid chart identity/rank")
			}
			ids[key] = true
		}
	case "market":
		if s.Market == nil || s.Market.Country != q.Country || s.Market.MinorUnit < 0 || s.Market.MinorUnit > 3 {
			return fmt.Errorf("invalid market")
		}
		v := s.Market
		if v.Status != "quoted" && v.Status != "no_quote" {
			return fmt.Errorf("invalid quote status")
		}
		unit, known := MinorUnit(v.Currency)
		if v.Status == "quoted" && (!known || unit != v.MinorUnit || !currency.MatchString(v.Currency) || v.AmountMinor == nil || *v.AmountMinor < 0) {
			return fmt.Errorf("invalid quote")
		}
		if v.Status == "no_quote" && (v.AmountMinor != nil || v.Currency != "") {
			return fmt.Errorf("no_quote carries money")
		}
		if v.MarketVerified && !SteamURL(v.EvidenceURL) {
			return fmt.Errorf("market requires context evidence")
		}
	case "fx":
		if s.Rates == nil || s.Rates.Provider != "ecb" || len(s.Rates.Items) == 0 || len(s.Rates.Items) > 100 {
			return fmt.Errorf("invalid rates")
		}
		ids := map[string]bool{}
		for _, v := range s.Rates.Items {
			key := v.Base + ":" + v.Quote
			r, ok := new(big.Rat).SetString(v.Decimal)
			_, e := time.Parse("2006-01-02", v.Date)
			if !currency.MatchString(v.Base) || v.Base != "EUR" || !currency.MatchString(v.Quote) || !decimal.MatchString(v.Decimal) || !ok || r.Sign() <= 0 || e != nil || ids[key] {
				return fmt.Errorf("invalid rate")
			}
			ids[key] = true
		}
	}
	b, e := json.Marshal(s)
	if e != nil || len(b) > MaxPayload {
		return fmt.Errorf("fact exceeds bounded payload")
	}
	return nil
}
func Decode(q Query, b []byte) (Snapshot, error) {
	var s Snapshot
	if len(b) == 0 || len(b) > MaxPayload {
		return s, fmt.Errorf("fact payload size")
	}
	if e := json.Unmarshal(b, &s); e != nil {
		return s, e
	}
	return s, s.Validate(q)
}

// ConvertMinor converts using EUR-based decimal rates and round-half-up. It
// never uses float money, and never substitutes a missing currency's rate.
func ConvertMinor(amount int64, fromUnit, toUnit int32, fromRate, toRate string) (string, error) {
	if amount < 0 || fromUnit < 0 || fromUnit > 3 || toUnit < 0 || toUnit > 3 {
		return "", fmt.Errorf("invalid money")
	}
	if !decimal.MatchString(fromRate) || !decimal.MatchString(toRate) {
		return "", fmt.Errorf("invalid decimal rate")
	}
	from, a := new(big.Rat).SetString(fromRate)
	to, b := new(big.Rat).SetString(toRate)
	if !a || !b || from.Sign() <= 0 || to.Sign() <= 0 {
		return "", fmt.Errorf("missing rate")
	}
	value := new(big.Rat).SetInt64(amount)
	value.Mul(value, to)
	value.Quo(value, from)
	power := func(unit int32) *big.Int {
		n := big.NewInt(1)
		for range unit {
			n.Mul(n, big.NewInt(10))
		}
		return n
	}
	value.Mul(value, new(big.Rat).SetInt(power(toUnit)))
	value.Quo(value, new(big.Rat).SetInt(power(fromUnit)))
	whole, rem := new(big.Int), new(big.Int)
	whole.QuoRem(value.Num(), value.Denom(), rem)
	if new(big.Int).Lsh(rem, 1).Cmp(value.Denom()) >= 0 {
		whole.Add(whole, big.NewInt(1))
	}
	return whole.String(), nil
}
func MinorUnit(code string) (int32, bool) {
	switch code {
	case "JPY", "KRW":
		return 0, true
	case "CNY", "USD", "GBP", "EUR":
		return 2, true
	}
	return 0, false
}
func AppString(id int64) string { return strconv.FormatInt(id, 10) }

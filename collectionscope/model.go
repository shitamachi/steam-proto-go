// Package collectionscope defines credential-free, versioned scheduling policy.
package collectionscope

import (
	"fmt"
	"math"
	"strings"
)

type Capability string
type SetKind string

const (
	Published SetKind = "published"
	Hot       SetKind = "hot"
	LongTail  SetKind = "long_tail"
	Explicit  SetKind = "explicit"
	Watched   SetKind = "watched"
)

type Spec struct {
	MinInterval int
	Countries   []string
	Cost        int
	Metadata    bool
	Language    string
}

var Specs = map[Capability]Spec{
	"players": {MinInterval: 1800, Cost: 1}, "news": {MinInterval: 21600, Cost: 5}, "achievements": {MinInterval: 604800, Cost: 1, Language: "schinese"}, "controller": {MinInterval: 604800, Cost: 1}, "market": {MinInterval: 21600, Cost: 1, Countries: []string{"CN", "US", "JP", "GB", "DE"}},
	"store_items": {MinInterval: 86400, Cost: 1, Metadata: true, Language: "schinese", Countries: []string{"CN"}}, "deck_report": {MinInterval: 604800, Cost: 1, Metadata: true}, "purchase_options": {MinInterval: 86400, Cost: 3, Metadata: true, Language: "schinese", Countries: []string{"CN"}},
}

type Set struct {
	ID     string  `json:"id"`
	Kind   SetKind `json:"kind"`
	AppIDs []int64 `json:"appIds"`
}
type Rule struct {
	ID               string     `json:"id"`
	SetID            string     `json:"setId"`
	Capability       Capability `json:"capability"`
	Enabled          bool       `json:"enabled"`
	Countries        []string   `json:"countries"`
	TargetLimit      int        `json:"targetLimit"`
	IntervalSeconds  int        `json:"intervalSeconds"`
	NegativeSeconds  int        `json:"negativeSeconds"`
	DailyBudget      int        `json:"dailyBudget"`
	FirstFillPercent int        `json:"firstFillPercent"`
}
type Config struct {
	Managed bool   `json:"managed"`
	Paused  bool   `json:"paused"`
	Sets    []Set  `json:"sets"`
	Rules   []Rule `json:"rules"`
}
type Patch struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	OperationKey    string `json:"operationKey"`
	Config          Config `json:"config"`
}
type Plan struct {
	RuleID        string     `json:"ruleId"`
	Capability    Capability `json:"capability"`
	SetID         string     `json:"setId"`
	Targets       int64      `json:"targets"`
	Touched       int64      `json:"touched"`
	DailyRequests int64      `json:"dailyRequests"`
	Budget        int64      `json:"budget"`
	ReservedToday int64      `json:"reservedToday"`
	InitialDays   int64      `json:"initialDays"`
	Status        string     `json:"status"`
	AppIDs        []int64    `json:"-"`
}
type Command struct {
	ExpectedVersion int64  `json:"expectedVersion"`
	OperationKey    string `json:"operationKey"`
	Action          string `json:"action"`
	RuleID          string `json:"ruleId"`
	AppID           int64  `json:"appId"`
	Country         string `json:"country"`
}

type Snapshot struct {
	Config     Config `json:"config"`
	Version    int64  `json:"version"`
	ObservedAt string `json:"observedAt"`
	Plans      []Plan `json:"plans"`
	Source     string `json:"source"`
}

func validID(v string) bool {
	if len(v) < 1 || len(v) > 40 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return false
		}
	}
	return true
}
func (c Config) Validate() error {
	if len(c.Sets) > 8 || len(c.Rules) > 12 {
		return fmt.Errorf("scope count exceeds bounds")
	}
	sets := map[string]bool{}
	total := 0
	for _, s := range c.Sets {
		if !validID(s.ID) || sets[s.ID] || len(s.AppIDs) > 1000 {
			return fmt.Errorf("invalid set")
		}
		sets[s.ID] = true
		explicit := s.Kind == Explicit || s.Kind == Watched
		if !explicit && s.Kind != Published && s.Kind != Hot && s.Kind != LongTail {
			return fmt.Errorf("invalid set kind")
		}
		if !explicit && len(s.AppIDs) > 0 {
			return fmt.Errorf("dynamic set cannot contain IDs")
		}
		seen := map[int64]bool{}
		for _, a := range s.AppIDs {
			if a < 1 || a > 2147483647 || seen[a] {
				return fmt.Errorf("invalid set AppID")
			}
			seen[a] = true
		}
		total += len(s.AppIDs)
	}
	if total > 4000 {
		return fmt.Errorf("explicit scope exceeds total bound")
	}
	ids := map[string]bool{}
	plannedScopes := 0
	for _, r := range c.Rules {
		sp, ok := Specs[r.Capability]
		if !ok || !validID(r.ID) || ids[r.ID] || !sets[r.SetID] || r.TargetLimit < 1 || r.TargetLimit > 1000 || r.IntervalSeconds < sp.MinInterval || r.IntervalSeconds > 90*86400 || r.NegativeSeconds < r.IntervalSeconds || r.NegativeSeconds > 90*86400 || r.DailyBudget < sp.Cost || r.DailyBudget > 100000 || r.FirstFillPercent < 10 || r.FirstFillPercent > 80 {
			return fmt.Errorf("invalid capability policy")
		}
		plannedScopes += r.TargetLimit * r.Dimensions()
		if plannedScopes > 20000 {
			return fmt.Errorf("combined scope exceeds query budget")
		}
		ids[r.ID] = true
		if len(sp.Countries) == 0 && len(r.Countries) > 0 || len(sp.Countries) > 0 && len(r.Countries) == 0 {
			return fmt.Errorf("invalid country dimension")
		}
		seen := map[string]bool{}
		for _, country := range r.Countries {
			if seen[country] || !strings.Contains(" "+strings.Join(sp.Countries, " ")+" ", " "+country+" ") {
				return fmt.Errorf("unsupported capability market")
			}
			seen[country] = true
		}
	}
	return nil
}
func (r Rule) Cost() int       { return Specs[r.Capability].Cost }
func (r Rule) Dimensions() int { return max(1, len(r.Countries)) }
func (r Rule) Estimate(targets int64) int64 {
	return int64(math.Ceil(float64(targets*int64(r.Dimensions()*r.Cost())) * 86400 / float64(r.IntervalSeconds)))
}
func (r Rule) FillDays(targets int64) int64 {
	n := targets * int64(r.Dimensions()*r.Cost())
	share := max(1, r.DailyBudget*r.FirstFillPercent/100)
	return (n + int64(share) - 1) / int64(share)
}

// Package dataquality is the credential-free, bounded Q2 quality wire contract.
// Content availability and freshness are independent partitions of a scope.
package dataquality

import (
	"fmt"
	f "github.com/shitamachi/steam-proto-go/externalfacts"
	"time"
)

type Scope struct {
	ConfiguredCount          *int64  `json:"configuredCount,omitempty"`
	RuleID                   string  `json:"ruleId,omitempty"`
	FreshnessSeconds         int     `json:"freshnessSeconds,omitempty"`
	NegativeFreshnessSeconds int     `json:"negativeFreshnessSeconds,omitempty"`
	Capability               string  `json:"capability"`
	Provider                 string  `json:"provider"`
	Country                  string  `json:"country"`
	Enabled                  bool    `json:"enabled"`
	AppIDs                   []int64 `json:"appIds,omitempty"`
	PublishedCatalog         bool    `json:"publishedCatalog"`
}
type Request struct {
	SchemaVersion int     `json:"schemaVersion"`
	ScopeVersion  string  `json:"scopeVersion"`
	Scopes        []Scope `json:"scopes"`
}

func (r Request) Validate() error {
	if r.SchemaVersion != 1 || len(r.ScopeVersion) > 100 || len(r.Scopes) > 128 {
		return fmt.Errorf("invalid quality scope")
	}
	seen := map[string]bool{}
	total := 0
	for _, s := range r.Scopes {
		if s.ConfiguredCount != nil && (*s.ConfiguredCount < 0 || *s.ConfiguredCount > 1000) {
			return fmt.Errorf("invalid configured count")
		}
		total += len(s.AppIDs)
		if total > 25000 {
			return fmt.Errorf("combined quality scope exceeds bound")
		}
		if TTL(s.Capability) == 0 || len(s.AppIDs) > 1000 || s.PublishedCatalog && len(s.AppIDs) > 0 || len(s.Provider) > 32 || len(s.RuleID) > 40 || s.FreshnessSeconds < 0 || s.FreshnessSeconds > 90*86400 || s.NegativeFreshnessSeconds < 0 || s.NegativeFreshnessSeconds > 90*86400 {
			return fmt.Errorf("invalid quality capability")
		}
		external := s.Capability == "external_identity" || s.Capability == "external_coverage" || s.Capability == "external_low" || s.Capability == "external_duration"
		if external {
			if s.PublishedCatalog || (s.Provider != "itad" && s.Provider != "igdb") || s.Provider == "itad" && !f.Market(s.Country) || s.Provider == "igdb" && (s.Country != "" || s.Capability != "external_identity" && s.Capability != "external_duration") || s.Provider == "itad" && s.Capability == "external_duration" {
				return fmt.Errorf("invalid external scope")
			}
		} else if s.Provider != "steam" || s.Country != "" && !f.Market(s.Country) {
			return fmt.Errorf("invalid steam scope")
		}
		key := s.RuleID + ":" + s.Capability + ":" + s.Provider + ":" + s.Country
		if seen[key] {
			return fmt.Errorf("duplicate quality scope")
		}
		seen[key] = true
		ids := map[int64]bool{}
		for _, a := range s.AppIDs {
			if a <= 0 || a > 2147483647 || ids[a] {
				return fmt.Errorf("invalid quality AppID")
			}
			ids[a] = true
		}
	}
	return nil
}

// TTL is a quality policy, never caller-controlled SQL or a provider guarantee.
func TTL(capability string) time.Duration {
	switch capability {
	case "price", "players":
		return 2 * time.Hour
	case "market":
		return 8 * time.Hour
	case "store_items", "news", "purchase_options":
		return 24 * time.Hour
	case "deck_report", "achievements", "controller":
		return 8 * 24 * time.Hour
	case "external_identity", "external_low":
		return 8 * 24 * time.Hour
	case "external_coverage":
		return 48 * time.Hour
	case "external_duration":
		return 14 * 24 * time.Hour
	}
	return 0
}

type Row struct {
	Scope
	Status           string     `json:"status"`
	Configured       int64      `json:"configured"`
	Eligible         int64      `json:"eligible"`
	Attempted        int64      `json:"attempted"`
	Available        int64      `json:"available"`
	Missing          int64      `json:"missing"`
	Ambiguous        int64      `json:"ambiguous"`
	Revoked          int64      `json:"revoked"`
	Other            int64      `json:"other"`
	Uncollected      int64      `json:"uncollected"`
	Fresh            int64      `json:"fresh"`
	Stale            int64      `json:"stale"`
	FreshnessUnknown int64      `json:"freshnessUnknown"`
	Quarantined      *int64     `json:"quarantined,omitempty"`
	LastFetchedAt    *time.Time `json:"lastFetchedAt,omitempty"`
	LastIngestedAt   *time.Time `json:"lastIngestedAt,omitempty"`
	MaxDelaySeconds  *float64   `json:"maxDelaySeconds,omitempty"`
}
type Report struct {
	SchemaVersion int        `json:"schemaVersion"`
	Stage         string     `json:"stage"`
	Status        string     `json:"status"`
	ScopeVersion  string     `json:"scopeVersion"`
	ObservedAt    *time.Time `json:"observedAt,omitempty"`
	Rows          []Row      `json:"rows"`
	Scopes        []Scope    `json:"scopes,omitempty"`
	Message       string     `json:"message,omitempty"`
}

// Add accepts one deduplicated eligible game; a nil status means not collected.
// No content or unknown timestamps are synthesized from the ingest time.
func (r *Row) Add(status *string, fetched, ingested *time.Time, now time.Time) {
	r.Eligible++
	if status == nil {
		r.Uncollected++
		return
	}
	r.Attempted++
	switch *status {
	case "present", "quoted":
		r.Available++
	case "missing", "no_quote":
		r.Missing++
	case "ambiguous":
		r.Ambiguous++
	case "revoked":
		r.Revoked++
	default:
		r.Other++
	}
	if fetched == nil || fetched.IsZero() || fetched.After(now.Add(time.Minute)) {
		r.FreshnessUnknown++
	} else {
		ttl := TTL(r.Capability)
		if r.FreshnessSeconds > 0 {
			ttl = time.Duration(r.FreshnessSeconds) * time.Second
		}
		if (*status == "missing" || *status == "no_quote" || *status == "ambiguous" && r.Capability == "controller") && r.NegativeFreshnessSeconds > 0 {
			ttl = time.Duration(r.NegativeFreshnessSeconds) * time.Second
		}
		if now.Sub(*fetched) <= ttl {
			r.Fresh++
		} else {
			r.Stale++
		}
		if r.LastFetchedAt == nil || fetched.After(*r.LastFetchedAt) {
			t := *fetched
			r.LastFetchedAt = &t
		}
		if ingested != nil && !ingested.Before(*fetched) && !ingested.After(now.Add(time.Minute)) {
			d := ingested.Sub(*fetched).Seconds()
			if r.MaxDelaySeconds == nil || d > *r.MaxDelaySeconds {
				r.MaxDelaySeconds = &d
			}
		}
	}
	if ingested != nil && !ingested.IsZero() && !ingested.After(now.Add(time.Minute)) && (r.LastIngestedAt == nil || ingested.After(*r.LastIngestedAt)) {
		t := *ingested
		r.LastIngestedAt = &t
	}
}

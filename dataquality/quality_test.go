package dataquality

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestContentAndTimePartitions(t *testing.T) {
	now := time.Now().UTC()
	old := now.Add(-9 * 24 * time.Hour)
	recent := now.Add(-time.Hour)
	future := now.Add(time.Hour)
	r := Row{Scope: Scope{Capability: "external_low"}}
	present, missing, revoked := "present", "missing", "revoked"
	r.Add(nil, nil, nil, now)
	r.Add(&present, nil, &now, now)
	r.Add(&missing, &old, &now, now)
	r.Add(&revoked, &recent, &now, now)
	r.Add(&present, &future, &now, now)
	if r.Eligible != 5 || r.Attempted != 4 || r.Uncollected != 1 || r.Available != 2 || r.Missing != 1 || r.Revoked != 1 || r.Fresh != 1 || r.Stale != 1 || r.FreshnessUnknown != 2 {
		t.Fatalf("incorrect partitions: %+v", r)
	}
	if r.LastFetchedAt == nil || !r.LastFetchedAt.Equal(recent) {
		t.Fatal("unknown or future time became latest fetch")
	}
}
func TestCacheCoalescesAndInvalidatesInFlight(t *testing.T) {
	var c Cache
	var calls atomic.Int32
	release := make(chan struct{})
	done := make(chan struct{})
	load := func(context.Context) (Report, error) {
		calls.Add(1)
		<-release
		close(done)
		at := time.Now()
		return Report{SchemaVersion: 1, Stage: "source", Status: "ready", ObservedAt: &at}, nil
	}
	for i := 0; i < 20; i++ {
		if c.Get("a", "source", load).Status != "pending" {
			t.Fatal("first refresh must be pending")
		}
	}
	c.Invalidate()
	close(release)
	<-done
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		busy := c.busy
		c.mu.Unlock()
		if !busy {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("refresh did not end")
		}
		time.Sleep(time.Millisecond)
	}
	if calls.Load() != 1 {
		t.Fatal("concurrent requests started duplicate reads")
	}
	c.mu.Lock()
	if c.report.Status != "pending" {
		t.Fatal("invalidated result published")
	}
	c.mu.Unlock()
}
func TestFailedRefreshPreservesObservedTime(t *testing.T) {
	at := time.Now().Add(-time.Hour)
	c := Cache{key: "a", report: Report{SchemaVersion: 1, Stage: "source", Status: "ready", ObservedAt: &at}}
	c.Get("a", "source", func(context.Context) (Report, error) { return Report{}, errors.New("db") })
	deadline := time.Now().Add(time.Second)
	for {
		c.mu.Lock()
		status := c.report.Status
		observed := c.report.ObservedAt
		c.mu.Unlock()
		if status == "stale" {
			if observed == nil || !observed.Equal(at) {
				t.Fatal("refresh failure changed observation")
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("did not retain stale snapshot")
		}
		time.Sleep(time.Millisecond)
	}
}
func TestRequestBounds(t *testing.T) {
	r := Request{SchemaVersion: 1, Scopes: []Scope{{Capability: "external_low", Provider: "itad", Country: "CN", AppIDs: []int64{570, 570}}}}
	if r.Validate() == nil {
		t.Fatal("duplicate IDs accepted")
	}
	r.Scopes[0].AppIDs = []int64{570}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	r.Scopes[0].PublishedCatalog = true
	if r.Validate() == nil {
		t.Fatal("external unbounded scope accepted")
	}
}

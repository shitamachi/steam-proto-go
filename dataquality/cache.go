package dataquality

import (
	"context"
	"sync"
	"time"
)

// Cache coalesces asynchronous refreshes. At most one bounded DB read runs per
// process. A changed scope key cannot expose the previous scope's result.
type Cache struct {
	mu         sync.Mutex
	key        string
	busy       bool
	generation uint64
	until      time.Time
	report     Report
}

func (c *Cache) Get(key, stage string, load func(context.Context) (Report, error)) Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.key != key {
		c.key = key
		c.generation++
		c.until = time.Time{}
		c.report = Report{SchemaVersion: 1, Stage: stage, Status: "pending", Rows: []Row{}}
	}
	if !c.busy && time.Now().After(c.until) {
		c.busy = true
		generation := c.generation
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			report, err := load(ctx)
			c.mu.Lock()
			defer c.mu.Unlock()
			c.busy = false
			if generation != c.generation {
				return
			}
			if err != nil {
				if c.report.ObservedAt == nil {
					c.report.Status = "unavailable"
				} else {
					c.report.Status = "stale"
				}
				c.report.Message = "quality_refresh_failed"
				c.until = time.Now().Add(time.Minute)
				return
			}
			c.report = report
			c.until = time.Now().Add(10 * time.Minute)
		}()
	}
	// Clone all slices/pointers through value ownership in the producer; consumers
	// must not mutate reports. Cache only replaces whole immutable reports.
	return c.report
}

// Invalidate discards snapshots and in-flight results after a configuration write.
func (c *Cache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.generation++
	c.until = time.Time{}
	c.report = Report{SchemaVersion: 1, Stage: c.report.Stage, Status: "pending", Rows: []Row{}}
}

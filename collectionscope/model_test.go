package collectionscope

import (
	"testing"
)

func policy() Config {
	return Config{Managed: true, Sets: []Set{{ID: "sample", Kind: Explicit, AppIDs: []int64{10, 20}}}, Rules: []Rule{{ID: "players", SetID: "sample", Capability: "players", Enabled: true, TargetLimit: 50, IntervalSeconds: 1800, NegativeSeconds: 86400, DailyBudget: 3000, FirstFillPercent: 60}}}
}
func TestPolicyBoundsAndRequestEstimate(t *testing.T) {
	c := policy()
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	r := c.Rules[0]
	if r.Estimate(50) != 2400 || r.FillDays(50) != 1 {
		t.Fatal("wrong request plan")
	}
	for _, alter := range []func(*Config){func(c *Config) { c.Rules[0].TargetLimit = 1001 }, func(c *Config) { c.Sets[0].AppIDs = []int64{10, 10} }, func(c *Config) { c.Rules[0].Capability = "external_identity" }, func(c *Config) { c.Rules[0].Countries = []string{"JP"} }, func(c *Config) { c.Rules[0].NegativeSeconds = 1 }, func(c *Config) { c.Rules[0].IntervalSeconds = 1 }, func(c *Config) { c.Sets[0].Kind = Published }, func(c *Config) { c.Rules[0].FirstFillPercent = 90 }} {
		c := policy()
		alter(&c)
		if c.Validate() == nil {
			t.Fatal("invalid policy accepted")
		}
	}
}
func TestColdAndHotCanHaveDistinctRefreshRules(t *testing.T) {
	c := policy()
	c.Sets = append(c.Sets, Set{ID: "hot", Kind: Hot})
	r := c.Rules[0]
	r.ID = "hot_players"
	r.SetID = "hot"
	c.Rules = append(c.Rules, r)
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
	c.Rules[1].ID = c.Rules[0].ID
	if c.Validate() == nil {
		t.Fatal("duplicate rule ID")
	}
}

func TestBudgetMustReserveOneCompleteTask(t *testing.T) {
	c := policy()
	r := &c.Rules[0]
	r.Capability = "news"
	r.IntervalSeconds = 21600
	r.DailyBudget = 4
	if c.Validate() == nil {
		t.Fatal("budget cannot admit one news task")
	}
	r.DailyBudget = 5
	if e := c.Validate(); e != nil {
		t.Fatal(e)
	}
}

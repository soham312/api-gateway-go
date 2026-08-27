package balancer

import (
	"testing"

	"github.com/soham312/api-gateway-go/internal/health"
)

func TestWeightedRoundRobin_Distribution(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b2 := health.NewBackend("http://b2", 2)
	wrr := NewWeightedRoundRobin([]*health.Backend{b1, b2})

	counts := map[string]int{}
	for i := 0; i < 30; i++ {
		b := wrr.Next()
		if b == nil {
			t.Fatalf("expected non-nil backend")
		}
		counts[b.URL]++
	}

	if counts["http://b1"] == 0 || counts["http://b2"] == 0 {
		t.Fatalf("expected both backends to be selected, got %v", counts)
	}
	if counts["http://b2"] <= counts["http://b1"] {
		t.Errorf("expected b2 (weight 2) to be selected more than b1 (weight 1), got %v", counts)
	}
}

func TestWeightedRoundRobin_Empty(t *testing.T) {
	wrr := NewWeightedRoundRobin(nil)
	if got := wrr.Next(); got != nil {
		t.Errorf("expected nil for empty backend list, got %v", got)
	}
}

func TestWeightedRoundRobin_SkipsUnhealthy(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b2 := health.NewBackend("http://b2", 1)
	b1.SetState(health.StateOpen)
	wrr := NewWeightedRoundRobin([]*health.Backend{b1, b2})

	for i := 0; i < 10; i++ {
		b := wrr.Next()
		if b == nil {
			t.Fatalf("expected non-nil backend")
		}
		if b.URL != "http://b2" {
			t.Errorf("expected only healthy backend b2 to be returned, got %s", b.URL)
		}
	}
}

func TestWeightedRoundRobin_AllUnhealthy(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b1.SetState(health.StateOpen)
	wrr := NewWeightedRoundRobin([]*health.Backend{b1})

	if got := wrr.Next(); got != nil {
		t.Errorf("expected nil when all backends unhealthy, got %v", got)
	}
}

func TestLeastConnections_PicksFewestConnections(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b2 := health.NewBackend("http://b2", 1)
	b1.RecordConnection()
	b1.RecordConnection()
	b2.RecordConnection()

	lc := NewLeastConnections([]*health.Backend{b1, b2})
	got := lc.Next()
	if got == nil || got.URL != "http://b2" {
		t.Errorf("expected b2 (fewer connections), got %v", got)
	}
}

func TestLeastConnections_SkipsUnhealthy(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b2 := health.NewBackend("http://b2", 1)
	b2.SetState(health.StateOpen)

	lc := NewLeastConnections([]*health.Backend{b1, b2})
	got := lc.Next()
	if got == nil || got.URL != "http://b1" {
		t.Errorf("expected only healthy backend b1, got %v", got)
	}
}

func TestLeastConnections_NoBackends(t *testing.T) {
	lc := NewLeastConnections(nil)
	if got := lc.Next(); got != nil {
		t.Errorf("expected nil for empty backend list, got %v", got)
	}
}

func TestLeastConnections_AllUnhealthy(t *testing.T) {
	b1 := health.NewBackend("http://b1", 1)
	b1.SetState(health.StateOpen)
	lc := NewLeastConnections([]*health.Backend{b1})
	if got := lc.Next(); got != nil {
		t.Errorf("expected nil when all backends unhealthy, got %v", got)
	}
}

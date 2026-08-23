package balancer

import (
	"sync/atomic"
	"github.com/soham312/api-gateway-go/internal/health"
)

type WeightedRoundRobin struct {
	backends []*health.Backend
	current  uint64
}

func NewWeightedRoundRobin(backends []*health.Backend) *WeightedRoundRobin {
	var unrolled []*health.Backend
	for _, b := range backends {
		for i := 0; i < b.Weight; i++ {
			unrolled = append(unrolled, b)
		}
	}
	return &WeightedRoundRobin{
		backends: unrolled,
	}
}

func (wrr *WeightedRoundRobin) Next() *health.Backend {
	if len(wrr.backends) == 0 {
		return nil
	}
	
	for i := 0; i < len(wrr.backends); i++ {
		idx := atomic.AddUint64(&wrr.current, 1) % uint64(len(wrr.backends))
		b := wrr.backends[idx]
		if b.IsHealthy() {
			return b
		}
	}
	return nil
}

package balancer

import (
	"sync/atomic"
	"github.com/soham312/api-gateway-go/internal/health"
)

type LeastConnections struct {
	backends []*health.Backend
}

func NewLeastConnections(backends []*health.Backend) *LeastConnections {
	return &LeastConnections{backends: backends}
}

func (lc *LeastConnections) Next() *health.Backend {
	var best *health.Backend
	var minConn int64 = -1

	for _, b := range lc.backends {
		if !b.IsHealthy() {
			continue
		}
		conns := atomic.LoadInt64(&b.ActiveConnections)
		if best == nil || conns < minConn {
			best = b
			minConn = conns
		}
	}
	return best
}

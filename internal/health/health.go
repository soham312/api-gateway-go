package health

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/soham312/api-gateway-go/internal/config"
)

type State int

const (
	StateClosed State = iota // Healthy
	StateOpen                // Unhealthy
	StateHalfOpen            // Testing recovery
)

type Backend struct {
	URL               string
	Weight            int
	ActiveConnections int64
	
	mu          sync.RWMutex
	state       State
	failures    int
	successes   int
}

func NewBackend(url string, weight int) *Backend {
	if weight <= 0 {
		weight = 1
	}
	return &Backend{
		URL:         url,
		Weight:      weight,
		state:       StateClosed,
	}
}

func (b *Backend) RecordConnection() {
	atomic.AddInt64(&b.ActiveConnections, 1)
}

func (b *Backend) ReleaseConnection() {
	atomic.AddInt64(&b.ActiveConnections, -1)
}

func (b *Backend) SetState(s State) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.state = s
}

func (b *Backend) GetState() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

func (b *Backend) IsHealthy() bool {
	s := b.GetState()
	return s == StateClosed || s == StateHalfOpen
}

func (b *Backend) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures = 0
	b.successes++
	
	cfg := config.Get()
	successThreshold := 1 // By default, 1 success recovers
	if cfg != nil && cfg.CBSuccessThreshold > 0 {
		successThreshold = cfg.CBSuccessThreshold
	}
	
	if (b.state == StateHalfOpen || b.state == StateOpen) && b.successes >= successThreshold {
		log.Printf("🟢 CIRCUIT CLOSED: %s is healthy.", b.URL)
		b.state = StateClosed
		b.successes = 0
	}
}

func (b *Backend) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	
	cfg := config.Get()
	maxFailures := 3
	if cfg != nil && cfg.CBFailureThreshold > 0 {
		maxFailures = cfg.CBFailureThreshold
	}

	if b.state == StateClosed && b.failures >= maxFailures {
		log.Printf("🔴 CIRCUIT TRIPPED: %s is down.", b.URL)
		b.state = StateOpen
	} else if b.state == StateHalfOpen {
		log.Printf("🔴 CIRCUIT TRIPPED: %s failed during half-open.", b.URL)
		b.state = StateOpen
	}
}

// Poller manages active health checks
type Poller struct {
	backends atomic.Value
}

func NewPoller(backends []*Backend) *Poller {
	p := &Poller{}
	p.UpdateBackends(backends)
	return p
}

func (p *Poller) UpdateBackends(backends []*Backend) {
	p.backends.Store(backends)
}

func (p *Poller) Start() {
	go func() {
		for {
			cfg := config.Get()
			healthPath := ""
			cbTimeout := 2 * time.Second
			if cfg != nil {
				if cfg.HealthCheckPath != "" {
					healthPath = cfg.HealthCheckPath
				}
				if cfg.CBTimeout != "" {
					if t, err := time.ParseDuration(cfg.CBTimeout); err == nil {
						cbTimeout = t
					}
				}
			}
			client := http.Client{Timeout: cbTimeout}
			if backends, ok := p.backends.Load().([]*Backend); ok {
				for _, b := range backends {
					state := b.GetState()
					
					if state == StateOpen {
						b.SetState(StateHalfOpen)
						log.Printf("🟡 CIRCUIT HALF-OPEN: %s testing recovery.", b.URL)
					}
					
					resp, err := client.Get(b.URL + healthPath)
					if err != nil || resp.StatusCode >= 500 {
						b.RecordFailure()
					} else {
						b.RecordSuccess()
					}
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

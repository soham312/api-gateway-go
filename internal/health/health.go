package health

import (
	"log"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
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
	maxFailures int
}

func NewBackend(url string, weight int) *Backend {
	if weight <= 0 {
		weight = 1
	}
	return &Backend{
		URL:         url,
		Weight:      weight,
		state:       StateClosed,
		maxFailures: 3,
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
	if b.state == StateHalfOpen || b.state == StateOpen {
		log.Printf("🟢 CIRCUIT CLOSED: %s is healthy.", b.URL)
		b.state = StateClosed
	}
}

func (b *Backend) RecordFailure() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures++
	if b.state == StateClosed && b.failures >= b.maxFailures {
		log.Printf("🔴 CIRCUIT TRIPPED: %s is down.", b.URL)
		b.state = StateOpen
	} else if b.state == StateHalfOpen {
		log.Printf("🔴 CIRCUIT TRIPPED: %s failed during half-open.", b.URL)
		b.state = StateOpen
	}
}

// Poller manages active health checks
type Poller struct {
	backends []*Backend
}

func NewPoller(backends []*Backend) *Poller {
	return &Poller{backends: backends}
}

func (p *Poller) Start() {
	go func() {
		client := http.Client{Timeout: 2 * time.Second}
		for {
			for _, b := range p.backends {
				state := b.GetState()
				
				if state == StateOpen {
					b.SetState(StateHalfOpen)
					log.Printf("🟡 CIRCUIT HALF-OPEN: %s testing recovery.", b.URL)
				}
				
				resp, err := client.Get(b.URL)
				if err != nil || resp.StatusCode >= 500 {
					b.RecordFailure()
				} else {
					b.RecordSuccess()
				}
			}
			time.Sleep(10 * time.Second)
		}
	}()
}

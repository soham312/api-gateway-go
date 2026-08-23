package proxy

import (
	"net/http/httputil"
	"sync"
)

// Backend represents a single target server.
type Backend struct {
	URL   string
	Alive bool
	Proxy *httputil.ReverseProxy
	mux   sync.RWMutex
}

// SetStatus safely updates the server's health state.
func (b *Backend) SetStatus(status bool) {
	b.mux.Lock()
	b.Alive = status
	b.mux.Unlock()
}

// IsAlive safely checks if the server is healthy.
func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	alive := b.Alive
	b.mux.RUnlock()
	return alive
}

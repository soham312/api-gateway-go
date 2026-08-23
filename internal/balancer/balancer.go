package balancer

import (
	"github.com/soham312/api-gateway-go/internal/health"
)

type Balancer interface {
	Next() *health.Backend
}

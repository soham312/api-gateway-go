package router

import (
	"sync/atomic"
	"strings"
	"github.com/soham312/api-gateway-go/internal/balancer"
)

type Route struct {
	Prefix      string
	StripPrefix bool
	Balancer    balancer.Balancer
}

type Router struct {
	routes atomic.Value
}

func NewRouter(routes []Route) *Router {
	r := &Router{}
	r.UpdateRoutes(routes)
	return r
}

func (r *Router) UpdateRoutes(routes []Route) {
	// Sort routes by prefix length descending for Longest-Prefix Match
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if len(routes[j].Prefix) > len(routes[i].Prefix) {
				routes[i], routes[j] = routes[j], routes[i]
			}
		}
	}
	r.routes.Store(routes)
}

func (r *Router) Match(path string) (*Route, string) {
	routes, ok := r.routes.Load().([]Route)
	if !ok {
		return nil, path
	}
	for i := range routes {
		route := &routes[i]
		if strings.HasPrefix(path, route.Prefix) {
			matchPath := path
			if route.StripPrefix {
				matchPath = strings.TrimPrefix(path, route.Prefix)
				if !strings.HasPrefix(matchPath, "/") {
					matchPath = "/" + matchPath
				}
			}
			return route, matchPath
		}
	}
	return nil, path
}

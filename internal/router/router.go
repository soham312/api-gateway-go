package router

import (
	"strings"
	"github.com/soham312/api-gateway-go/internal/balancer"
)

type Route struct {
	Prefix      string
	StripPrefix bool
	Balancer    balancer.Balancer
}

type Router struct {
	routes []Route
}

func NewRouter(routes []Route) *Router {
	// Sort routes by prefix length descending for Longest-Prefix Match
	for i := 0; i < len(routes); i++ {
		for j := i + 1; j < len(routes); j++ {
			if len(routes[j].Prefix) > len(routes[i].Prefix) {
				routes[i], routes[j] = routes[j], routes[i]
			}
		}
	}
	return &Router{routes: routes}
}

func (r *Router) Match(path string) (*Route, string) {
	for i := range r.routes {
		route := &r.routes[i]
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

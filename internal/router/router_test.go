package router

import "testing"

func TestRouter_LongestPrefixMatch(t *testing.T) {
	routes := []Route{
		{Prefix: "/api", StripPrefix: false},
		{Prefix: "/api/v1", StripPrefix: false},
	}
	r := NewRouter(routes)

	route, _ := r.Match("/api/v1/users")
	if route == nil || route.Prefix != "/api/v1" {
		t.Fatalf("expected longest prefix /api/v1 to win, got %+v", route)
	}
}

func TestRouter_StripPrefix(t *testing.T) {
	routes := []Route{
		{Prefix: "/users", StripPrefix: true},
	}
	r := NewRouter(routes)

	route, path := r.Match("/users/123")
	if route == nil {
		t.Fatalf("expected a route match")
	}
	if path != "/123" {
		t.Errorf("expected stripped path /123, got %s", path)
	}
}

func TestRouter_StripPrefixExactMatch(t *testing.T) {
	routes := []Route{
		{Prefix: "/users", StripPrefix: true},
	}
	r := NewRouter(routes)

	route, path := r.Match("/users")
	if route == nil {
		t.Fatalf("expected a route match")
	}
	if path != "/" {
		t.Errorf("expected stripped path to be normalized to /, got %q", path)
	}
}

func TestRouter_NoStripPrefix(t *testing.T) {
	routes := []Route{
		{Prefix: "/products", StripPrefix: false},
	}
	r := NewRouter(routes)

	route, path := r.Match("/products/42")
	if route == nil {
		t.Fatalf("expected a route match")
	}
	if path != "/products/42" {
		t.Errorf("expected unmodified path, got %s", path)
	}
}

func TestRouter_NoMatch(t *testing.T) {
	routes := []Route{
		{Prefix: "/users", StripPrefix: true},
	}
	r := NewRouter(routes)

	route, path := r.Match("/unknown")
	if route != nil {
		t.Errorf("expected no route match, got %+v", route)
	}
	if path != "/unknown" {
		t.Errorf("expected original path returned on no match, got %s", path)
	}
}

func TestRouter_UpdateRoutes(t *testing.T) {
	r := NewRouter([]Route{{Prefix: "/a", StripPrefix: false}})
	route, _ := r.Match("/a/b")
	if route == nil {
		t.Fatalf("expected initial route match")
	}

	r.UpdateRoutes([]Route{{Prefix: "/b", StripPrefix: false}})
	route, _ = r.Match("/a/b")
	if route != nil {
		t.Errorf("expected old route to be gone after update, got %+v", route)
	}
	route, _ = r.Match("/b/c")
	if route == nil {
		t.Errorf("expected new route to match after update")
	}
}

func TestRouter_EmptyRouter(t *testing.T) {
	r := &Router{}
	route, path := r.Match("/anything")
	if route != nil {
		t.Errorf("expected nil route on uninitialized router, got %+v", route)
	}
	if path != "/anything" {
		t.Errorf("expected original path returned, got %s", path)
	}
}

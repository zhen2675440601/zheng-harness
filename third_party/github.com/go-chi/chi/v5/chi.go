package chi

import (
	"context"
	"net/http"
	"strings"
)

type Router interface {
	http.Handler
	Use(middlewares ...func(http.Handler) http.Handler)
	Route(pattern string, fn func(Router))
	Get(pattern string, handlerFn http.HandlerFunc)
	Post(pattern string, handlerFn http.HandlerFunc)
}

type routeContextKey struct{}

type route struct {
	method  string
	pattern string
	handler http.Handler
}

type Mux struct {
	prefix      string
	routes      *[]route
	middlewares []func(http.Handler) http.Handler
}

func NewRouter() *Mux {
	routes := make([]route, 0)
	return &Mux{routes: &routes}
}

func (m *Mux) Use(middlewares ...func(http.Handler) http.Handler) {
	m.middlewares = append(m.middlewares, middlewares...)
}

func (m *Mux) Route(pattern string, fn func(Router)) {
	child := &Mux{prefix: joinPattern(m.prefix, pattern), routes: m.routes, middlewares: append([]func(http.Handler) http.Handler(nil), m.middlewares...)}
	fn(child)
}

func (m *Mux) Get(pattern string, handlerFn http.HandlerFunc) {
	m.handle(http.MethodGet, pattern, handlerFn)
}

func (m *Mux) Post(pattern string, handlerFn http.HandlerFunc) {
	m.handle(http.MethodPost, pattern, handlerFn)
}

func (m *Mux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range *m.routes {
		params, ok := matchPattern(route.pattern, r.URL.Path)
		if !ok || route.method != r.Method {
			continue
		}
		ctx := context.WithValue(r.Context(), routeContextKey{}, params)
		route.handler.ServeHTTP(w, r.WithContext(ctx))
		return
	}
	http.NotFound(w, r)
}

func URLParam(r *http.Request, key string) string {
	if r == nil {
		return ""
	}
	params, _ := r.Context().Value(routeContextKey{}).(map[string]string)
	return params[key]
}

func (m *Mux) handle(method, pattern string, handler http.Handler) {
	wrapped := handler
	for i := len(m.middlewares) - 1; i >= 0; i-- {
		wrapped = m.middlewares[i](wrapped)
	}
	*m.routes = append(*m.routes, route{method: method, pattern: joinPattern(m.prefix, pattern), handler: wrapped})
}

func joinPattern(prefix, pattern string) string {
	base := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	child := strings.TrimSpace(pattern)
	if child == "" {
		child = "/"
	}
	if !strings.HasPrefix(child, "/") {
		child = "/" + child
	}
	if base == "" || base == "/" {
		return child
	}
	return base + child
}

func matchPattern(pattern, path string) (map[string]string, bool) {
	patternParts := splitPattern(pattern)
	pathParts := splitPattern(path)
	if len(patternParts) != len(pathParts) {
		return nil, false
	}
	params := make(map[string]string)
	for i := range patternParts {
		part := patternParts[i]
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			params[strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")] = pathParts[i]
			continue
		}
		if part != pathParts[i] {
			return nil, false
		}
	}
	return params, true
}

func splitPattern(value string) []string {
	trimmed := strings.Trim(strings.TrimSpace(value), "/")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "/")
}

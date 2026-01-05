package routing

import (
	"net/http"
	"runtime/debug"
)

type Router struct {
	classifier *Classifier
	routes     map[string]map[string]routeEntry
}

type routeEntry struct {
	rc      RouteClass
	handler http.Handler
}

func NewRouter(classifier *Classifier) *Router {
	return &Router{
		classifier: classifier,
		routes:     make(map[string]map[string]routeEntry),
	}
}

func (r *Router) Handle(rc RouteClass, method string, path string, h http.Handler) {
	if r.routes[path] == nil {
		r.routes[path] = make(map[string]routeEntry)
	}

	r.routes[path][method] = routeEntry{
		rc: rc,
		handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					_ = debug.Stack()
					WriteError(w, req, rc, http.StatusInternalServerError, "internal_error", "internal error")
				}
			}()
			h.ServeHTTP(w, req)
		}),
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	methods, ok := r.routes[req.URL.Path]
	if !ok {
		WriteError(w, req, r.classifier.Classify(req.URL.Path), http.StatusNotFound, "not_found", "not found")
		return
	}
	entry, ok := methods[req.Method]
	if !ok {
		WriteError(w, req, entrypointClass(methods, r.classifier.Classify(req.URL.Path)), http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	entry.handler.ServeHTTP(w, req)
}

func entrypointClass(methods map[string]routeEntry, fallback RouteClass) RouteClass {
	for _, e := range methods {
		return e.rc
	}
	return fallback
}

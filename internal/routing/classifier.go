package routing

import (
	"errors"
	"strings"
)

type RouteClass string

const (
	RouteClassUI          RouteClass = "ui"
	RouteClassInternalAPI RouteClass = "internal_api"
	RouteClassPublicAPI   RouteClass = "public_api"
	RouteClassWebhook     RouteClass = "webhook"
	RouteClassAuthn       RouteClass = "authn"
	RouteClassOps         RouteClass = "ops"
	RouteClassDevOnly     RouteClass = "dev_only"
	RouteClassTestOnly    RouteClass = "test_only"
	RouteClassStatic      RouteClass = "static"
	RouteClassWebsocket   RouteClass = "websocket"
)

type Classifier struct {
	entrypoint        string
	allowExact        map[string]RouteClass
	allowPathPatterns []pathPatternRoute
}

func NewClassifier(a Allowlist, entrypoint string) (*Classifier, error) {
	ep, ok := a.Entrypoints[entrypoint]
	if !ok {
		return nil, errors.New("allowlist: missing entrypoint")
	}
	if len(ep.Routes) == 0 {
		return nil, errors.New("allowlist: entrypoint routes empty")
	}

	exact := make(map[string]RouteClass, len(ep.Routes))
	var patterns []pathPatternRoute
	for _, r := range ep.Routes {
		if r.Path == "" || r.RouteClass == "" {
			return nil, errors.New("allowlist: invalid route")
		}
		if p, ok := parsePathPattern(r.Path); ok {
			patterns = append(patterns, pathPatternRoute{pattern: p, rc: RouteClass(r.RouteClass)})
			continue
		}
		exact[r.Path] = RouteClass(r.RouteClass)
	}
	return &Classifier{entrypoint: entrypoint, allowExact: exact, allowPathPatterns: patterns}, nil
}

func (c *Classifier) Classify(path string) RouteClass {
	if rc, ok := c.allowExact[path]; ok {
		return rc
	}
	for _, p := range c.allowPathPatterns {
		if p.pattern.Match(path) {
			return p.rc
		}
	}

	switch {
	case hasPrefixSegment(path, "/api/v1"):
		return RouteClassPublicAPI
	case isModuleInternalAPI(path):
		return RouteClassInternalAPI
	case hasPrefixSegment(path, "/webhooks"):
		return RouteClassWebhook
	case hasPrefixSegment(path, "/_dev"):
		return RouteClassDevOnly
	case hasPrefixSegment(path, "/__test__"):
		return RouteClassTestOnly
	case hasPrefixSegment(path, "/assets") || hasPrefixSegment(path, "/static") || hasPrefixSegment(path, "/uploads"):
		return RouteClassStatic
	case path == "/ws":
		return RouteClassWebsocket
	default:
		return RouteClassUI
	}
}

func hasPrefixSegment(path, prefix string) bool {
	if path == prefix {
		return true
	}
	return strings.HasPrefix(path, prefix+"/")
}

func isModuleInternalAPI(path string) bool {
	// /{module}/api/*
	// segment-boundary: module must be a single segment.
	if !strings.HasPrefix(path, "/") {
		return false
	}
	rest := strings.TrimPrefix(path, "/")
	module, after, ok := strings.Cut(rest, "/")
	if !ok || module == "" {
		return false
	}
	return hasPrefixSegment("/"+after, "/api")
}

type pathPatternRoute struct {
	pattern PathPattern
	rc      RouteClass
}

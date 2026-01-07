package routing

import "strings"

type PathPattern struct {
	raw      string
	segments []string
}

func parsePathPattern(raw string) (PathPattern, bool) {
	if !strings.Contains(raw, "{") {
		return PathPattern{}, false
	}
	if raw == "" || raw[0] != '/' {
		return PathPattern{}, false
	}

	parts := splitPathSegments(raw)
	for _, s := range parts {
		if s == "" {
			return PathPattern{}, false
		}
		if strings.Contains(s, "{") || strings.Contains(s, "}") {
			if !isParamSegment(s) {
				return PathPattern{}, false
			}
		}
	}
	return PathPattern{raw: raw, segments: parts}, true
}

func (p PathPattern) Match(path string) bool {
	if p.raw == "" {
		return false
	}
	in := splitPathSegments(path)
	if len(in) != len(p.segments) {
		return false
	}
	for i := range p.segments {
		want := p.segments[i]
		got := in[i]
		if got == "" {
			return false
		}
		if isParamSegment(want) {
			continue
		}
		if got != want {
			return false
		}
	}
	return true
}

func splitPathSegments(path string) []string {
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

func isParamSegment(s string) bool {
	return strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}") && len(s) > 2
}

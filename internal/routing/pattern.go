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
				if _, _, ok := splitPatternSegment(s); !ok {
					return PathPattern{}, false
				}
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
			if strings.ContainsRune(got, ':') {
				return false
			}
			continue
		}
		if prefix, suffix, ok := splitPatternSegment(want); ok {
			if len(got) <= len(prefix)+len(suffix) {
				return false
			}
			if !strings.HasPrefix(got, prefix) || !strings.HasSuffix(got, suffix) {
				return false
			}
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

func splitPatternSegment(s string) (prefix string, suffix string, ok bool) {
	open := strings.IndexByte(s, '{')
	close := strings.IndexByte(s, '}')
	if open < 0 || close < 0 || close <= open {
		return "", "", false
	}
	if strings.Contains(s[close+1:], "{") || strings.Contains(s[:open], "}") {
		return "", "", false
	}
	name := strings.TrimSpace(s[open+1 : close])
	if name == "" {
		return "", "", false
	}
	if strings.ContainsRune(name, '{') || strings.ContainsRune(name, '}') {
		return "", "", false
	}
	if strings.Contains(s[close+1:], "}") {
		return "", "", false
	}
	return s[:open], s[close+1:], true
}

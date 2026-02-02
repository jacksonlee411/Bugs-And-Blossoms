package routing

import "testing"

func TestClassifier_SegmentBoundary(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server": {Routes: []Route{{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"}}},
		},
	}
	c, err := NewClassifier(a, "server")
	if err != nil {
		t.Fatal(err)
	}

	if got := c.Classify("/api/v1"); got != RouteClassPublicAPI {
		t.Fatalf("got=%q", got)
	}
	if got := c.Classify("/api/v1x"); got == RouteClassPublicAPI {
		t.Fatalf("unexpected public api: %q", got)
	}
	if got := c.Classify("/org/api"); got != RouteClassInternalAPI {
		t.Fatalf("got=%q", got)
	}
	if got := c.Classify("/org/apix"); got == RouteClassInternalAPI {
		t.Fatalf("unexpected internal api: %q", got)
	}

	if got := c.Classify("orgunit/api"); got != RouteClassUI {
		t.Fatalf("got=%q", got)
	}
	if got := c.Classify("/"); got != RouteClassUI {
		t.Fatalf("got=%q", got)
	}
}

func TestNewClassifier_Errors(t *testing.T) {
	t.Parallel()

	_, err := NewClassifier(Allowlist{Version: 1, Entrypoints: map[string]Entrypoint{"server": {Routes: nil}}}, "server")
	if err == nil {
		t.Fatal("expected empty routes error")
	}

	_, err = NewClassifier(Allowlist{Version: 1, Entrypoints: map[string]Entrypoint{"server": {Routes: []Route{{}}}}}, "server")
	if err == nil {
		t.Fatal("expected invalid route error")
	}
}

func TestClassifier_AllClasses(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"server": {Routes: []Route{
				{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"},
				{Path: "/login", Methods: []string{"GET"}, RouteClass: "authn"},
			}},
		},
	}
	c, err := NewClassifier(a, "server")
	if err != nil {
		t.Fatal(err)
	}

	cases := map[string]RouteClass{
		"/login":          RouteClassAuthn,
		"/webhooks/foo/x": RouteClassWebhook,
		"/_dev/x":         RouteClassDevOnly,
		"/__test__/x":     RouteClassTestOnly,
		"/assets/x":       RouteClassStatic,
		"/static/x":       RouteClassStatic,
		"/uploads/x":      RouteClassStatic,
		"/ws":             RouteClassWebsocket,
		"/anything-else":  RouteClassUI,
	}
	for path, want := range cases {
		if got := c.Classify(path); got != want {
			t.Fatalf("path=%s got=%q want=%q", path, got, want)
		}
	}
}

func TestClassifier_PathPattern(t *testing.T) {
	t.Parallel()

	a := Allowlist{
		Version: 1,
		Entrypoints: map[string]Entrypoint{
			"superadmin": {Routes: []Route{
				{Path: "/health", Methods: []string{"GET"}, RouteClass: "ops"},
				{Path: "/superadmin/tenants/{tenant_id}/disable", Methods: []string{"POST"}, RouteClass: "ui"},
			}},
		},
	}
	c, err := NewClassifier(a, "superadmin")
	if err != nil {
		t.Fatal(err)
	}

	if got := c.Classify("/superadmin/tenants/abc/disable"); got != RouteClassUI {
		t.Fatalf("got=%q", got)
	}
}

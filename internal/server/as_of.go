package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

const asOfLayout = "2006-01-02"

func currentUTCDateString() string {
	return time.Now().UTC().Format(asOfLayout)
}

func requireAsOf(w http.ResponseWriter, r *http.Request) (string, bool) {
	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			q := r.URL.Query()
			q.Set("as_of", currentUTCDateString())
			u := *r.URL
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusFound)
			return "", false
		}
		asOf = currentUTCDateString()
	}

	if _, err := time.Parse(asOfLayout, asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return "", false
	}

	return asOf, true
}

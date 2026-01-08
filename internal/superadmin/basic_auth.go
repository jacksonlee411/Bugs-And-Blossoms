package superadmin

import (
	"net/http"
	"os"
)

func withBasicAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/healthz":
			next.ServeHTTP(w, r)
			return
		default:
		}

		user := os.Getenv("SUPERADMIN_BASIC_AUTH_USER")
		pass := os.Getenv("SUPERADMIN_BASIC_AUTH_PASS")
		if user == "" || pass == "" {
			next.ServeHTTP(w, r)
			return
		}

		gotUser, gotPass, ok := r.BasicAuth()
		if !ok || gotUser != user || gotPass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="superadmin"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized\n"))
			return
		}

		next.ServeHTTP(w, r)
	})
}

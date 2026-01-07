package superadmin

import (
	"context"
	"net/http"
	"os"
)

type actorCtxKey struct{}

func actorFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(actorCtxKey{}).(string)
	return v, ok
}

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
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized\n"))
			return
		}

		gotUser, gotPass, ok := r.BasicAuth()
		if !ok || gotUser != user || gotPass != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="superadmin"`)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("unauthorized\n"))
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), actorCtxKey{}, gotUser))
		next.ServeHTTP(w, r)
	})
}

package superadmin

import (
	"context"
	"net/http"
)

type principalCtxKey struct{}

func principalFromContext(ctx context.Context) (superadminPrincipal, bool) {
	v, ok := ctx.Value(principalCtxKey{}).(superadminPrincipal)
	return v, ok
}

func withSuperadminSession(store sessionStore, principals principalStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health", "/healthz", "/superadmin/login":
			next.ServeHTTP(w, r)
			return
		case "/superadmin/logout":
			next.ServeHTTP(w, r)
			return
		default:
		}

		saSid, ok := readSASID(r)
		if !ok {
			http.Redirect(w, r, "/superadmin/login", http.StatusFound)
			return
		}

		sess, found, err := store.Lookup(r.Context(), saSid)
		if err != nil {
			http.Error(w, "internal error\n", http.StatusInternalServerError)
			return
		}
		if !found {
			clearSASIDCookie(w)
			http.Redirect(w, r, "/superadmin/login", http.StatusFound)
			return
		}

		p, ok, err := principals.GetByID(r.Context(), sess.PrincipalID)
		if err != nil {
			http.Error(w, "internal error\n", http.StatusInternalServerError)
			return
		}
		if !ok || p.Status != "active" {
			_ = store.Revoke(r.Context(), saSid)
			clearSASIDCookie(w)
			http.Redirect(w, r, "/superadmin/login", http.StatusFound)
			return
		}

		r = r.WithContext(context.WithValue(r.Context(), principalCtxKey{}, p))
		next.ServeHTTP(w, r)
	})
}

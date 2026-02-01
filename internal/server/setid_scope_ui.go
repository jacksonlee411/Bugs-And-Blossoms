package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

func handleSetIDScopeSubscriptionsUI(w http.ResponseWriter, r *http.Request, store SetIDGovernanceStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	setID := parseScopeSubscriptionSetID(r.URL.Path)
	if setID == "" {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_request", "setid required")
		return
	}

	asOf := strings.TrimSpace(r.URL.Query().Get("as_of"))
	if asOf == "" {
		asOf = time.Now().UTC().Format("2006-01-02")
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusBadRequest, "invalid_as_of", "invalid as_of")
		return
	}

	switch r.Method {
	case http.MethodGet:
		htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, "")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(htmlOut))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, "bad form")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlOut))
			return
		}
		scopeCode := strings.TrimSpace(r.Form.Get("scope_code"))
		packageID := strings.TrimSpace(r.Form.Get("package_id"))
		effectiveDate := strings.TrimSpace(r.Form.Get("effective_date"))
		requestID := strings.TrimSpace(r.Form.Get("request_code"))
		if effectiveDate == "" {
			effectiveDate = asOf
		}
		if scopeCode == "" || packageID == "" || requestID == "" {
			htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, "scope_code/package_id/request_code required")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlOut))
			return
		}
		if _, err := time.Parse("2006-01-02", effectiveDate); err != nil {
			htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, "effective_date invalid")
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlOut))
			return
		}
		if _, err := store.CreateScopeSubscription(r.Context(), tenant.ID, setID, scopeCode, packageID, "tenant", effectiveDate, requestID, tenant.ID); err != nil {
			htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, stablePgMessage(err))
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(htmlOut))
			return
		}
		w.Header().Set("HX-Trigger", fmt.Sprintf(`{"scopeSubscriptionChanged":{"setid":"%s","scope_code":"%s"}}`, setID, scopeCode))
		htmlOut := renderScopeSubscriptionsPartial(r, store, tenant.ID, setID, asOf, "")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(htmlOut))
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func renderScopeSubscriptionsPartial(r *http.Request, store SetIDGovernanceStore, tenantID string, setID string, asOf string, errMsg string) string {
	var b strings.Builder
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(errMsg) + `</p>`)
	}

	scopes, err := store.ListScopeCodes(r.Context(), tenantID)
	if err != nil {
		b.WriteString(`<p style="color:#b00">` + html.EscapeString(err.Error()) + `</p>`)
		return b.String()
	}

	type scopeRow struct {
		scope ScopeCode
		sub   ScopeSubscription
		ok    bool
		pkgs  []ScopePackage
	}

	rows := make([]scopeRow, 0, len(scopes))
	for _, s := range scopes {
		row := scopeRow{scope: s}
		if s.ShareMode != "shared-only" {
			pkgs, _ := store.ListScopePackages(r.Context(), tenantID, s.ScopeCode)
			row.pkgs = pkgs
		}
		sub, err := store.GetScopeSubscription(r.Context(), tenantID, setID, s.ScopeCode, asOf)
		if err == nil {
			row.sub = sub
			row.ok = true
		}
		rows = append(rows, row)
	}

	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>` +
		`<th>scope_code</th><th>share_mode</th><th>current_package</th><th>effective_date</th><th>end_date</th><th>action</th>` +
		`</tr></thead><tbody>`)

	for _, row := range rows {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(row.scope.ScopeCode) + "</td>")
		b.WriteString("<td>" + html.EscapeString(row.scope.ShareMode) + "</td>")

		packageLabel := "(missing)"
		effectiveDate := ""
		endDate := ""
		if row.ok {
			effectiveDate = row.sub.EffectiveDate
			endDate = row.sub.EndDate
			packageLabel = row.sub.PackageID
			if row.sub.PackageOwner == "tenant" {
				for _, p := range row.pkgs {
					if p.PackageID == row.sub.PackageID {
						packageLabel = p.PackageCode
						break
					}
				}
			}
		}
		b.WriteString("<td>" + html.EscapeString(packageLabel) + "</td>")
		b.WriteString("<td>" + html.EscapeString(effectiveDate) + "</td>")
		b.WriteString("<td>" + html.EscapeString(endDate) + "</td>")

		if row.scope.ShareMode == "shared-only" {
			b.WriteString("<td>(read-only)</td>")
		} else {
			formAction := "/orgunit/setids/" + url.PathEscape(setID) + "/scope-subscriptions?as_of=" + url.QueryEscape(asOf)
			reqID := fmt.Sprintf("ui:scope-sub:%s:%s:%d", setID, row.scope.ScopeCode, time.Now().UTC().UnixNano())
			b.WriteString(`<td><form method="POST" action="` + html.EscapeString(formAction) + `" hx-post="` + html.EscapeString(formAction) + `" hx-target="#scope-subscriptions" hx-swap="innerHTML">`)
			b.WriteString(`<input type="hidden" name="scope_code" value="` + html.EscapeString(row.scope.ScopeCode) + `" />`)
			b.WriteString(`<label>Package <select name="package_id">`)
			b.WriteString(`<option value="">(select)</option>`)
			for _, p := range row.pkgs {
				selected := ""
				if row.ok && p.PackageID == row.sub.PackageID {
					selected = " selected"
				}
				b.WriteString(`<option value="` + html.EscapeString(p.PackageID) + `"` + selected + `>` + html.EscapeString(p.PackageCode) + `</option>`)
			}
			b.WriteString(`</select></label> `)
			b.WriteString(`<label>Effective Date <input type="date" name="effective_date" value="` + html.EscapeString(asOf) + `" /></label> `)
			b.WriteString(`<input type="hidden" name="request_code" value="` + html.EscapeString(reqID) + `" />`)
			b.WriteString(`<button type="submit">Subscribe</button>`)
			b.WriteString(`</form></td>`)
		}
		b.WriteString("</tr>")
	}
	b.WriteString("</tbody></table>")

	return b.String()
}

func parseScopeSubscriptionSetID(path string) string {
	const prefix = "/orgunit/setids/"
	const suffix = "/scope-subscriptions"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return ""
	}
	trimmed := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	return strings.Trim(trimmed, "/")
}

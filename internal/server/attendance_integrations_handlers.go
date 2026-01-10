package server

import (
	"errors"
	"html"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
)

func handleAttendanceIntegrations(w http.ResponseWriter, r *http.Request, personStore PersonStore) {
	tenant, ok := currentTenant(r.Context())
	if !ok {
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "tenant_missing", "tenant missing")
		return
	}

	asOf, ok := requireAsOf(w, r)
	if !ok {
		return
	}

	load := func() ([]Person, []ExternalIdentityLink, string) {
		persons, err := personStore.ListPersons(r.Context(), tenant.ID)
		if err != nil {
			return nil, nil, err.Error()
		}
		links, err := personStore.ListExternalIdentityLinks(r.Context(), tenant.ID, 2000)
		if err != nil {
			return persons, nil, err.Error()
		}
		return persons, links, ""
	}

	switch r.Method {
	case http.MethodGet:
		persons, links, errMsg := load()
		writePage(w, r, renderAttendanceIntegrations(tenant, asOf, persons, links, errMsg))
		return
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			persons, links, _ := load()
			writePageWithStatus(w, r, http.StatusUnprocessableEntity, renderAttendanceIntegrations(tenant, asOf, persons, links, "bad form"))
			return
		}

		if _, ok := currentPrincipal(r.Context()); !ok {
			routing.WriteError(w, r, routing.RouteClassUI, http.StatusInternalServerError, "principal_missing", "principal missing")
			return
		}

		op := strings.TrimSpace(r.Form.Get("op"))
		provider := strings.TrimSpace(r.Form.Get("provider"))
		externalUserID := strings.TrimSpace(r.Form.Get("external_user_id"))

		var err error
		switch op {
		case "link":
			personUUID := strings.TrimSpace(r.Form.Get("person_uuid"))
			err = personStore.LinkExternalIdentity(r.Context(), tenant.ID, provider, externalUserID, personUUID)
		case "disable":
			err = personStore.DisableExternalIdentity(r.Context(), tenant.ID, provider, externalUserID)
		case "enable":
			err = personStore.EnableExternalIdentity(r.Context(), tenant.ID, provider, externalUserID)
		case "ignore":
			err = personStore.IgnoreExternalIdentity(r.Context(), tenant.ID, provider, externalUserID)
		case "unignore":
			err = personStore.UnignoreExternalIdentity(r.Context(), tenant.ID, provider, externalUserID)
		case "unlink":
			err = personStore.UnlinkExternalIdentity(r.Context(), tenant.ID, provider, externalUserID)
		default:
			err = errors.New("unsupported op")
		}

		if err != nil {
			persons, links, errMsg := load()
			writePageWithStatus(w, r, http.StatusUnprocessableEntity, renderAttendanceIntegrations(tenant, asOf, persons, links, mergeMsg(errMsg, err.Error())))
			return
		}

		http.Redirect(w, r, "/org/attendance-integrations?as_of="+url.QueryEscape(asOf), http.StatusSeeOther)
		return
	default:
		routing.WriteError(w, r, routing.RouteClassUI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
}

func renderAttendanceIntegrations(tenant Tenant, asOf string, persons []Person, links []ExternalIdentityLink, errMsg string) string {
	var pending []ExternalIdentityLink
	var active []ExternalIdentityLink
	var disabled []ExternalIdentityLink
	var ignored []ExternalIdentityLink

	for _, l := range links {
		switch l.Status {
		case "pending":
			pending = append(pending, l)
		case "active":
			active = append(active, l)
		case "disabled":
			disabled = append(disabled, l)
		case "ignored":
			ignored = append(ignored, l)
		default:
		}
	}

	byPerson := make(map[string]Person, len(persons))
	for _, p := range persons {
		byPerson[p.UUID] = p
	}

	sort.Slice(pending, func(i, j int) bool { return pending[i].LastSeenAt.After(pending[j].LastSeenAt) })
	sort.Slice(active, func(i, j int) bool { return active[i].LastSeenAt.After(active[j].LastSeenAt) })
	sort.Slice(disabled, func(i, j int) bool { return disabled[i].LastSeenAt.After(disabled[j].LastSeenAt) })
	sort.Slice(ignored, func(i, j int) bool { return ignored[i].LastSeenAt.After(ignored[j].LastSeenAt) })

	var b strings.Builder
	b.WriteString("<h1>Attendance Integrations</h1>")
	b.WriteString("<p>Tenant: " + html.EscapeString(tenant.Name) + "</p>")
	b.WriteString("<p>MVP: only identity mapping + ingestion readiness (no credentials hosting).</p>")
	if errMsg != "" {
		b.WriteString(`<p style="color:#b00020">` + html.EscapeString(errMsg) + `</p>`)
	}

	b.WriteString(`<h2>Link (create or update)</h2>`)
	b.WriteString(`<form method="POST" action="/org/attendance-integrations?as_of=` + url.QueryEscape(asOf) + `">`)
	b.WriteString(`<input type="hidden" name="op" value="link">`)
	b.WriteString(`<label>Provider <select name="provider"><option value="DINGTALK">DINGTALK</option><option value="WECOM">WECOM</option></select></label><br/>`)
	b.WriteString(`<label>External User ID <input name="external_user_id" /></label><br/>`)
	b.WriteString(`<label>Person <select name="person_uuid">`)
	for _, p := range persons {
		label := p.DisplayName
		if label == "" {
			label = p.Pernr
		} else if p.Pernr != "" {
			label = label + " (" + p.Pernr + ")"
		}
		b.WriteString(`<option value="` + html.EscapeString(p.UUID) + `">` + html.EscapeString(label) + `</option>`)
	}
	b.WriteString(`</select></label><br/>`)
	b.WriteString(`<button type="submit">Link</button>`)
	b.WriteString(`</form>`)

	b.WriteString(`<h2>Pending</h2>`)
	b.WriteString(renderExternalIdentityLinksTable(asOf, pending, byPerson, persons))

	b.WriteString(`<h2>Active</h2>`)
	b.WriteString(renderExternalIdentityLinksTable(asOf, active, byPerson, persons))

	b.WriteString(`<h2>Disabled</h2>`)
	b.WriteString(renderExternalIdentityLinksTable(asOf, disabled, byPerson, persons))

	b.WriteString(`<h2>Ignored</h2>`)
	b.WriteString(renderExternalIdentityLinksTable(asOf, ignored, byPerson, persons))

	return b.String()
}

func renderExternalIdentityLinksTable(asOf string, links []ExternalIdentityLink, byPerson map[string]Person, persons []Person) string {
	var b strings.Builder
	b.WriteString(`<table border="1" cellspacing="0" cellpadding="6"><thead><tr>`)
	b.WriteString(`<th>provider</th><th>external_user_id</th><th>status</th><th>person</th><th>last_seen_at (UTC)</th><th>seen_count</th><th>actions</th>`)
	b.WriteString(`</tr></thead><tbody>`)

	for _, l := range links {
		b.WriteString(`<tr>`)
		b.WriteString(`<td>` + html.EscapeString(l.Provider) + `</td>`)
		b.WriteString(`<td><code>` + html.EscapeString(l.ExternalUserID) + `</code></td>`)
		b.WriteString(`<td>` + html.EscapeString(l.Status) + `</td>`)

		personLabel := ""
		if l.PersonUUID != nil && *l.PersonUUID != "" {
			if p, ok := byPerson[*l.PersonUUID]; ok {
				personLabel = p.DisplayName
				if p.Pernr != "" {
					personLabel = personLabel + " (" + p.Pernr + ")"
				}
			} else {
				personLabel = *l.PersonUUID
			}
		}
		if personLabel == "" {
			personLabel = "-"
		}
		b.WriteString(`<td>` + html.EscapeString(personLabel) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(l.LastSeenAt.Format(time.RFC3339)) + `</td>`)
		b.WriteString(`<td>` + html.EscapeString(strconv.Itoa(l.SeenCount)) + `</td>`)

		b.WriteString(`<td>`)
		action := func(op string, extra string, label string) {
			b.WriteString(`<form method="POST" action="/org/attendance-integrations?as_of=` + url.QueryEscape(asOf) + `" style="display:inline;margin-right:6px">`)
			b.WriteString(`<input type="hidden" name="op" value="` + html.EscapeString(op) + `">`)
			b.WriteString(`<input type="hidden" name="provider" value="` + html.EscapeString(l.Provider) + `">`)
			b.WriteString(`<input type="hidden" name="external_user_id" value="` + html.EscapeString(l.ExternalUserID) + `">`)
			if extra != "" {
				b.WriteString(extra)
			}
			b.WriteString(`<button type="submit">` + html.EscapeString(label) + `</button>`)
			b.WriteString(`</form>`)
		}

		switch l.Status {
		case "pending":
			var selectPerson strings.Builder
			selectPerson.WriteString(`<select name="person_uuid">`)
			for _, p := range persons {
				label := p.DisplayName
				if label == "" {
					label = p.Pernr
				} else if p.Pernr != "" {
					label = label + " (" + p.Pernr + ")"
				}
				selectPerson.WriteString(`<option value="` + html.EscapeString(p.UUID) + `">` + html.EscapeString(label) + `</option>`)
			}
			selectPerson.WriteString(`</select>`)
			action("link", selectPerson.String(), "Link")
			action("ignore", "", "Ignore")
		case "active":
			action("disable", "", "Disable")
			action("unlink", "", "Unlink")
		case "disabled":
			action("enable", "", "Enable")
			action("unlink", "", "Unlink")
		case "ignored":
			action("unignore", "", "Unignore")
		default:
		}

		b.WriteString(`</td>`)
		b.WriteString(`</tr>`)
	}

	b.WriteString(`</tbody></table>`)
	return b.String()
}

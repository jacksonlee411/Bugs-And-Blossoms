package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	orgunitpkg "github.com/jacksonlee411/Bugs-And-Blossoms/pkg/orgunit"
)

type setIDResolvedOrgRef struct {
	OrgCode    string
	OrgNodeKey string
}

func resolveSetIDOrgCodeRef(ctx context.Context, tenantID string, orgCode string, orgResolver OrgUnitCodeResolver) (setIDResolvedOrgRef, error) {
	normalizedOrgCode, err := orgunitpkg.NormalizeOrgCode(orgCode)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	if orgResolver == nil {
		return setIDResolvedOrgRef{}, errors.New("org code resolver missing")
	}
	orgNodeKey, err := orgResolver.ResolveOrgNodeKeyByCode(ctx, tenantID, normalizedOrgCode)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	normalizedOrgNodeKey, err := normalizeOrgNodeKeyInput(orgNodeKey)
	if err != nil {
		return setIDResolvedOrgRef{}, err
	}
	return setIDResolvedOrgRef{
		OrgCode:    normalizedOrgCode,
		OrgNodeKey: normalizedOrgNodeKey,
	}, nil
}

func resolveSetIDExplainOrgCode(ctx context.Context, tenantID string, orgCode string, orgResolver OrgUnitCodeResolver) (string, string, error) {
	ref, err := resolveSetIDOrgCodeRef(ctx, tenantID, orgCode, orgResolver)
	if err != nil {
		return "", "", err
	}
	return ref.OrgCode, ref.OrgNodeKey, nil
}

func writeSetIDExplainOrgCodeError(w http.ResponseWriter, r *http.Request, field string, err error) {
	switch {
	case errors.Is(err, orgunitpkg.ErrOrgCodeInvalid):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusBadRequest, field+"_invalid", field+" invalid")
	case errors.Is(err, orgunitpkg.ErrOrgCodeNotFound):
		routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusNotFound, field+"_not_found", field+" not found")
	default:
		writeInternalAPIError(w, r, err, "setid_explain_"+field+"_resolve_failed")
	}
}

func traceIDFromRequestHeader(r *http.Request) string {
	traceparent := strings.TrimSpace(r.Header.Get("traceparent"))
	if traceparent == "" {
		return ""
	}
	parts := strings.Split(traceparent, "-")
	if len(parts) != 4 {
		return ""
	}
	traceID := strings.ToLower(parts[1])
	if len(traceID) != 32 || traceID == "00000000000000000000000000000000" {
		return ""
	}
	for _, ch := range traceID {
		if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
			return ""
		}
	}
	return traceID
}

func normalizeSetIDExplainRequestID(r *http.Request) string {
	requestID := strings.TrimSpace(r.URL.Query().Get("request_id"))
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("X-Request-Id"))
	}
	if requestID == "" {
		requestID = strings.TrimSpace(r.Header.Get("x-request-id"))
	}
	if requestID != "" {
		return requestID
	}
	if traceID := traceIDFromRequestHeader(r); traceID != "" {
		return "trace-" + traceID
	}
	return "setid-explain-auto"
}

func fallbackSetIDExplainTraceID(requestID string, capabilityKey string, businessUnitOrgCode string, asOf string) string {
	sum := sha256.Sum256([]byte(strings.Join([]string{
		strings.TrimSpace(requestID),
		strings.ToLower(strings.TrimSpace(capabilityKey)),
		strings.TrimSpace(businessUnitOrgCode),
		strings.TrimSpace(asOf),
	}, "|")))
	return hex.EncodeToString(sum[:16])
}

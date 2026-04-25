package cubebox

import (
	"regexp"
	"strings"
	"time"
)

var pageContextSlugPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,63}$`)
var pageContextEntityPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

type PageContext struct {
	Page           string             `json:"page,omitempty"`
	BusinessObject string             `json:"business_object,omitempty"`
	CurrentObject  *PageObjectContext `json:"current_object,omitempty"`
	View           *PageViewContext   `json:"view,omitempty"`
}

type PageObjectContext struct {
	Domain    string `json:"domain,omitempty"`
	EntityKey string `json:"entity_key,omitempty"`
	Label     string `json:"label,omitempty"`
}

type PageViewContext struct {
	AsOf string `json:"as_of,omitempty"`
}

func NormalizePageContext(value *PageContext) *PageContext {
	if value == nil {
		return nil
	}
	normalized := PageContext{
		Page:           normalizePagePath(value.Page),
		BusinessObject: normalizePageSlug(value.BusinessObject),
		CurrentObject:  normalizePageObjectContext(value.CurrentObject),
		View:           normalizePageViewContext(value.View),
	}
	if normalized.Page == "" && normalized.BusinessObject == "" && normalized.CurrentObject == nil && normalized.View == nil {
		return nil
	}
	return &normalized
}

func normalizePageObjectContext(value *PageObjectContext) *PageObjectContext {
	if value == nil {
		return nil
	}
	normalized := PageObjectContext{
		Domain:    normalizePageSlug(value.Domain),
		EntityKey: normalizePageEntityKey(value.EntityKey),
		Label:     normalizePageLabel(value.Label),
	}
	if normalized.Domain == "" && normalized.EntityKey == "" && normalized.Label == "" {
		return nil
	}
	return &normalized
}

func normalizePageViewContext(value *PageViewContext) *PageViewContext {
	if value == nil {
		return nil
	}
	asOf := strings.TrimSpace(value.AsOf)
	if asOf == "" {
		return nil
	}
	if _, err := time.Parse("2006-01-02", asOf); err != nil {
		return nil
	}
	return &PageViewContext{AsOf: asOf}
}

func normalizePagePath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if index := strings.IndexAny(value, "?#"); index >= 0 {
		value = value[:index]
	}
	if !strings.HasPrefix(value, "/") {
		return ""
	}
	if len(value) > 160 {
		value = value[:160]
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return ""
	}
	return value
}

func normalizePageSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || !pageContextSlugPattern.MatchString(value) {
		return ""
	}
	return value
}

func normalizePageEntityKey(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !pageContextEntityPattern.MatchString(value) {
		return ""
	}
	return value
}

func normalizePageLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if strings.ContainsAny(value, "\r\n\t") {
		return ""
	}
	if len([]rune(value)) > 120 {
		value = string([]rune(value)[:120])
	}
	return strings.TrimSpace(value)
}

package cubebox

import (
	"slices"
	"strings"
)

type APIToolRequestSchema struct {
	Required []string                `json:"required"`
	Optional []string                `json:"optional"`
	Params   map[string]APIParamSpec `json:"params"`
}

type APIParamSpec struct {
	Type        string `json:"type"`
	Description string `json:"description,omitempty"`
}

type APIToolObservationProjection struct {
	RootField       string   `json:"root_field,omitempty"`
	SummaryFields   []string `json:"summary_fields,omitempty"`
	EntityKeyField  string   `json:"entity_key_field,omitempty"`
	EntityNameField string   `json:"entity_name_field,omitempty"`
}

type APITool struct {
	Method                string                       `json:"method"`
	Path                  string                       `json:"path"`
	OperationID           string                       `json:"operation_id"`
	UseSummary            string                       `json:"use_summary"`
	RequestSchema         APIToolRequestSchema         `json:"request_schema"`
	ResponseSchemaRef     string                       `json:"response_schema_ref"`
	ObservationProjection APIToolObservationProjection `json:"observation_projection"`
	ResourceObject        string                       `json:"resource_object"`
	Action                string                       `json:"action"`
	AuthzCapabilityKey    string                       `json:"authz_capability_key"`
}

func (t APITool) Normalized() APITool {
	t.Method = strings.ToUpper(strings.TrimSpace(t.Method))
	t.Path = normalizeAPICallPath(t.Path)
	t.OperationID = strings.TrimSpace(t.OperationID)
	t.UseSummary = strings.TrimSpace(t.UseSummary)
	t.ResponseSchemaRef = strings.TrimSpace(t.ResponseSchemaRef)
	t.ResourceObject = strings.TrimSpace(t.ResourceObject)
	t.Action = strings.TrimSpace(t.Action)
	t.AuthzCapabilityKey = strings.TrimSpace(t.AuthzCapabilityKey)
	t.RequestSchema.Required = normalizeParamNames(t.RequestSchema.Required)
	t.RequestSchema.Optional = normalizeParamNames(t.RequestSchema.Optional)
	if t.RequestSchema.Params == nil {
		t.RequestSchema.Params = map[string]APIParamSpec{}
	}
	t.ObservationProjection.RootField = strings.TrimSpace(t.ObservationProjection.RootField)
	t.ObservationProjection.SummaryFields = normalizeParamNames(t.ObservationProjection.SummaryFields)
	t.ObservationProjection.EntityKeyField = strings.TrimSpace(t.ObservationProjection.EntityKeyField)
	t.ObservationProjection.EntityNameField = strings.TrimSpace(t.ObservationProjection.EntityNameField)
	return t
}

func (t APITool) AllowsParam(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	return slices.Contains(t.RequestSchema.Required, name) || slices.Contains(t.RequestSchema.Optional, name)
}

func (t APITool) RequiresParam(name string) bool {
	return slices.Contains(t.RequestSchema.Required, strings.TrimSpace(name))
}

func APIToolRouteID(method string, path string) string {
	return strings.ToUpper(strings.TrimSpace(method)) + " " + normalizeAPICallPath(path)
}

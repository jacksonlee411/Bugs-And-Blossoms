package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/routing"
	"gopkg.in/yaml.v3"
)

const (
	defaultAssistantRuntimeVersionsLockPath = "deploy/librechat/versions.lock.yaml"
	defaultAssistantRuntimeStatusPath       = "deploy/librechat/runtime-status.json"
)

const (
	assistantRuntimeHealthHealthy         = "healthy"
	assistantRuntimeHealthDegraded        = "degraded"
	assistantRuntimeHealthUnavailable     = "unavailable"
	assistantRuntimeHealthRetired         = "retired"
	assistantRuntimeReasonRetiredByDesign = "retired_by_design"
)

type assistantRuntimeStatusResponse struct {
	Status       string                         `json:"status"`
	CheckedAt    string                         `json:"checked_at"`
	ErrorCode    string                         `json:"error_code,omitempty"`
	ErrorMessage string                         `json:"error_message,omitempty"`
	Upstream     assistantRuntimeUpstreamStatus `json:"upstream"`
	Services     []assistantRuntimeService      `json:"services"`
	Capabilities assistantRuntimeCapabilities   `json:"capabilities"`
	Code         string                         `json:"code,omitempty"`
	Message      string                         `json:"message,omitempty"`
}

type assistantRuntimeUpstreamStatus struct {
	URL         string `json:"url,omitempty"`
	Repo        string `json:"repo,omitempty"`
	Ref         string `json:"ref,omitempty"`
	ImportedAt  string `json:"imported_at,omitempty"`
	RollbackRef string `json:"rollback_ref,omitempty"`
}

type assistantRuntimeService struct {
	Name     string `json:"name"`
	Required bool   `json:"required"`
	Healthy  string `json:"healthy"`
	Reason   string `json:"reason,omitempty"`
	Image    string `json:"image,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Digest   string `json:"digest,omitempty"`
}

type assistantRuntimeVersionsLock struct {
	Upstream struct {
		Repo        string `yaml:"repo"`
		Ref         string `yaml:"ref"`
		ImportedAt  string `yaml:"imported_at"`
		RollbackRef string `yaml:"rollback_ref"`
	} `yaml:"upstream"`
	Services []struct {
		Name     string `yaml:"name"`
		Required bool   `yaml:"required"`
		Image    string `yaml:"image"`
		Tag      string `yaml:"tag"`
		Digest   string `yaml:"digest"`
	} `yaml:"services"`
}

func handleAssistantRuntimeStatusAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		routingWriteMethodNotAllowed(w, r)
		return
	}
	status := assistantRuntimeStatus()
	httpStatus := http.StatusOK
	if status.Status == assistantRuntimeHealthUnavailable {
		httpStatus = http.StatusServiceUnavailable
		status.Code = status.ErrorCode
		status.Message = status.ErrorMessage
	}
	writeJSON(w, httpStatus, status)
}

func routingWriteMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	routing.WriteError(w, r, routing.RouteClassInternalAPI, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
}

func assistantRuntimeStatus() assistantRuntimeStatusResponse {
	resp := assistantRuntimeStatusResponse{
		Status:       assistantRuntimeHealthUnavailable,
		CheckedAt:    time.Now().UTC().Format(time.RFC3339Nano),
		Capabilities: assistantRuntimeFormalCapabilityMatrix(),
	}
	resp.Upstream.URL = assistantRuntimeDefaultUpstreamURL()
	if _, err := url.ParseRequestURI(resp.Upstream.URL); err != nil {
		resp.ErrorCode = "ai_runtime_config_invalid"
		resp.ErrorMessage = "assistant runtime upstream is invalid"
		resp.Services = []assistantRuntimeService{
			{
				Name:     "api",
				Required: true,
				Healthy:  assistantRuntimeHealthUnavailable,
				Reason:   "upstream_invalid",
			},
		}
		return resp
	}

	lock, lockErr := readAssistantRuntimeVersionsLock()
	if lockErr != nil {
		resp.ErrorCode = assistantRuntimeLockReadErrorCode(lockErr)
		resp.ErrorMessage = "assistant runtime versions lock is unavailable"
	} else {
		resp.Upstream.Repo = strings.TrimSpace(lock.Upstream.Repo)
		resp.Upstream.Ref = strings.TrimSpace(lock.Upstream.Ref)
		resp.Upstream.ImportedAt = strings.TrimSpace(lock.Upstream.ImportedAt)
		resp.Upstream.RollbackRef = strings.TrimSpace(lock.Upstream.RollbackRef)
		resp.Services = assistantRuntimeServicesFromLock(lock.Services)
	}

	snapshot, snapshotErr := readAssistantRuntimeSnapshot()
	if snapshotErr == nil {
		if strings.TrimSpace(snapshot.CheckedAt) != "" {
			resp.CheckedAt = snapshot.CheckedAt
		}
		resp.Status = normalizeAssistantRuntimeHealth(snapshot.Status)
		resp.Services = mergeAssistantRuntimeServices(resp.Services, snapshot.Services)
	}

	if len(resp.Services) == 0 {
		resp.Services = []assistantRuntimeService{
			{
				Name:     "api",
				Required: true,
				Healthy:  assistantRuntimeHealthUnavailable,
				Reason:   "status_snapshot_missing",
			},
		}
	}

	resp = assistantRuntimeApplyUpstreamProbe(resp)
	resp.Status = assistantRuntimeAggregateStatus(resp.Services)
	if capabilities, err := assistantRuntimeCapabilitiesStatus(); err == nil {
		resp.Capabilities = capabilities
	} else {
		resp.Status = assistantRuntimeHealthUnavailable
		resp.ErrorCode = assistantRuntimeDomainPolicyErrorCode(err)
		resp.ErrorMessage = assistantRuntimeDomainPolicyErrorMessage(err)
	}
	if resp.Status == assistantRuntimeHealthUnavailable && resp.ErrorCode == "" {
		resp.ErrorCode = "assistant_runtime_dependency_unavailable"
		resp.ErrorMessage = "assistant runtime dependencies are unavailable"
	}
	return resp
}

func assistantRuntimeDomainPolicyErrorCode(err error) string {
	switch {
	case errors.Is(err, errAssistantDomainPolicyMissing):
		return "assistant_oss_domain_policy_missing"
	default:
		return "assistant_oss_domain_policy_invalid"
	}
}

func assistantRuntimeDomainPolicyErrorMessage(err error) string {
	switch {
	case errors.Is(err, errAssistantDomainPolicyMissing):
		return "assistant domain allowlist policy is missing"
	default:
		return "assistant domain allowlist policy is invalid"
	}
}

func assistantRuntimeServicesFromLock(services []struct {
	Name     string `yaml:"name"`
	Required bool   `yaml:"required"`
	Image    string `yaml:"image"`
	Tag      string `yaml:"tag"`
	Digest   string `yaml:"digest"`
}) []assistantRuntimeService {
	if len(services) == 0 {
		return nil
	}
	out := make([]assistantRuntimeService, 0, len(services))
	for _, service := range services {
		name := strings.TrimSpace(service.Name)
		if name == "" {
			continue
		}
		out = append(out, assistantRuntimeService{
			Name:     name,
			Required: service.Required,
			Healthy:  assistantRuntimeHealthUnavailable,
			Reason:   "status_snapshot_missing",
			Image:    strings.TrimSpace(service.Image),
			Tag:      strings.TrimSpace(service.Tag),
			Digest:   strings.TrimSpace(service.Digest),
		})
	}
	for idx := range out {
		out[idx] = assistantRuntimeNormalizeService(out[idx])
	}
	return out
}

func mergeAssistantRuntimeServices(base []assistantRuntimeService, snapshot []assistantRuntimeService) []assistantRuntimeService {
	if len(base) == 0 && len(snapshot) == 0 {
		return nil
	}
	if len(base) == 0 {
		out := make([]assistantRuntimeService, len(snapshot))
		copy(out, snapshot)
		for idx := range out {
			out[idx] = assistantRuntimeNormalizeService(out[idx])
		}
		return out
	}

	merged := make([]assistantRuntimeService, len(base))
	copy(merged, base)
	byName := make(map[string]int, len(merged))
	for idx := range merged {
		byName[strings.ToLower(strings.TrimSpace(merged[idx].Name))] = idx
	}
	for _, service := range snapshot {
		nameKey := strings.ToLower(strings.TrimSpace(service.Name))
		if nameKey == "" {
			continue
		}
		service = assistantRuntimeNormalizeService(service)
		if idx, ok := byName[nameKey]; ok {
			merged[idx].Required = service.Required
			merged[idx].Healthy = service.Healthy
			merged[idx].Reason = strings.TrimSpace(service.Reason)
			if merged[idx].Image == "" {
				merged[idx].Image = strings.TrimSpace(service.Image)
			}
			if merged[idx].Tag == "" {
				merged[idx].Tag = strings.TrimSpace(service.Tag)
			}
			if merged[idx].Digest == "" {
				merged[idx].Digest = strings.TrimSpace(service.Digest)
			}
			continue
		}
		merged = append(merged, service)
	}
	return merged
}

func assistantRuntimeApplyUpstreamProbe(resp assistantRuntimeStatusResponse) assistantRuntimeStatusResponse {
	if strings.TrimSpace(resp.Upstream.URL) == "" {
		return resp
	}
	if err := assistantRuntimeProbeUpstream(resp.Upstream.URL); err != nil {
		resp.Services = assistantRuntimeUpsertService(resp.Services, assistantRuntimeService{
			Name:     "api",
			Required: true,
			Healthy:  assistantRuntimeHealthUnavailable,
			Reason:   "upstream_unreachable",
		})
		if resp.ErrorCode == "" {
			resp.ErrorCode = assistantUIProxyUpstreamUnavailable
			resp.ErrorMessage = "assistant runtime upstream is unreachable"
		}
	}
	return resp
}

func assistantRuntimeProbeUpstream(rawURL string) error {
	requestURL, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	req, _ := http.NewRequest(http.MethodGet, requestURL.String(), nil)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func assistantRuntimeUpsertService(services []assistantRuntimeService, next assistantRuntimeService) []assistantRuntimeService {
	nameKey := strings.ToLower(strings.TrimSpace(next.Name))
	if nameKey == "" {
		return services
	}
	next = assistantRuntimeNormalizeService(next)
	for idx := range services {
		if strings.ToLower(strings.TrimSpace(services[idx].Name)) != nameKey {
			continue
		}
		services[idx].Required = next.Required
		services[idx].Healthy = next.Healthy
		services[idx].Reason = strings.TrimSpace(next.Reason)
		return services
	}
	return append(services, next)
}

func assistantRuntimeAggregateStatus(services []assistantRuntimeService) string {
	if len(services) == 0 {
		return assistantRuntimeHealthUnavailable
	}
	hasDegraded := false
	for _, service := range services {
		healthy := normalizeAssistantRuntimeHealth(service.Healthy)
		if strings.EqualFold(strings.TrimSpace(service.Reason), assistantRuntimeReasonRetiredByDesign) {
			continue
		}
		if service.Required && healthy != assistantRuntimeHealthHealthy {
			return assistantRuntimeHealthUnavailable
		}
		if !service.Required && healthy != assistantRuntimeHealthHealthy {
			hasDegraded = true
		}
	}
	if hasDegraded {
		return assistantRuntimeHealthDegraded
	}
	return assistantRuntimeHealthHealthy
}

func normalizeAssistantRuntimeHealth(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case assistantRuntimeHealthHealthy:
		return assistantRuntimeHealthHealthy
	case assistantRuntimeHealthDegraded:
		return assistantRuntimeHealthDegraded
	case assistantRuntimeHealthRetired:
		return assistantRuntimeHealthRetired
	default:
		return assistantRuntimeHealthUnavailable
	}
}

func assistantRuntimeNormalizeService(service assistantRuntimeService) assistantRuntimeService {
	service.Healthy = normalizeAssistantRuntimeHealth(service.Healthy)
	service.Reason = strings.TrimSpace(service.Reason)
	if strings.EqualFold(service.Reason, assistantRuntimeReasonRetiredByDesign) {
		service.Required = false
		service.Healthy = assistantRuntimeHealthRetired
	}
	return service
}

func assistantRuntimeDefaultUpstreamURL() string {
	target := strings.TrimSpace(os.Getenv("LIBRECHAT_UPSTREAM"))
	if target != "" {
		return target
	}
	port := strings.TrimSpace(os.Getenv("LIBRECHAT_PORT"))
	if port == "" {
		port = "3080"
	}
	return "http://127.0.0.1:" + port
}

func assistantRuntimeLockReadErrorCode(err error) string {
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "assistant_runtime_versions_lock_missing"
	default:
		return "assistant_runtime_versions_lock_invalid"
	}
}

func readAssistantRuntimeVersionsLock() (assistantRuntimeVersionsLock, error) {
	var lock assistantRuntimeVersionsLock
	path := assistantRuntimeResolvePath(strings.TrimSpace(os.Getenv("ASSISTANT_RUNTIME_VERSIONS_LOCK")), defaultAssistantRuntimeVersionsLockPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return lock, err
	}
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return lock, err
	}
	return lock, nil
}

func readAssistantRuntimeSnapshot() (assistantRuntimeStatusResponse, error) {
	var snapshot assistantRuntimeStatusResponse
	path := assistantRuntimeResolvePath(strings.TrimSpace(os.Getenv("ASSISTANT_RUNTIME_STATUS_FILE")), defaultAssistantRuntimeStatusPath)
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func assistantRuntimeResolvePath(path, fallback string) string {
	candidate := strings.TrimSpace(path)
	if candidate == "" {
		candidate = fallback
	}
	if filepath.IsAbs(candidate) {
		return candidate
	}
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	probe := candidate
	for range 8 {
		probe = filepath.Join("..", probe)
		if _, err := os.Stat(probe); err == nil {
			return probe
		}
	}
	return candidate
}

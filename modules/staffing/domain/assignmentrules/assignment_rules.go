package assignmentrules

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jacksonlee411/Bugs-And-Blossoms/pkg/httperr"
)

var assignmentCorrectionNamespace = uuid.Must(uuid.Parse("28ed309c-cec7-406c-a442-eef4ef9034ce"))
var assignmentRescindNamespace = uuid.Must(uuid.Parse("fd58b41a-6ccc-451c-b9b4-cb924810fb2d"))

type PreparedUpsertPrimaryAssignment struct {
	EffectiveDate string
	PersonUUID    string
	PositionUUID  string
	Status        string
	AllocatedFTE  string
}

type PreparedCorrectAssignmentEvent struct {
	AssignmentUUID      string
	TargetEffectiveDate string
	CanonicalPayload    []byte
	EventID             string
}

type PreparedRescindAssignmentEvent struct {
	AssignmentUUID      string
	TargetEffectiveDate string
	CanonicalPayload    []byte
	EventID             string
}

func PrepareUpsertPrimaryAssignment(effectiveDate string, personUUID string, positionUUID string, status string, allocatedFte string) (PreparedUpsertPrimaryAssignment, error) {
	effectiveDate = strings.TrimSpace(effectiveDate)
	if effectiveDate == "" {
		return PreparedUpsertPrimaryAssignment{}, errors.New("effective_date is required")
	}
	personUUID = strings.TrimSpace(personUUID)
	if personUUID == "" {
		return PreparedUpsertPrimaryAssignment{}, errors.New("person_uuid is required")
	}
	positionUUID = strings.TrimSpace(positionUUID)
	if positionUUID == "" {
		return PreparedUpsertPrimaryAssignment{}, errors.New("position_uuid is required")
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = "active"
	}

	return PreparedUpsertPrimaryAssignment{
		EffectiveDate: effectiveDate,
		PersonUUID:    personUUID,
		PositionUUID:  positionUUID,
		Status:        status,
		AllocatedFTE:  strings.TrimSpace(allocatedFte),
	}, nil
}

func PrepareCorrectAssignmentEvent(tenantID string, assignmentUUID string, targetEffectiveDate string, replacementPayload json.RawMessage) (PreparedCorrectAssignmentEvent, error) {
	assignmentUUID = strings.TrimSpace(assignmentUUID)
	if assignmentUUID == "" {
		return PreparedCorrectAssignmentEvent{}, httperr.NewBadRequest("assignment_uuid is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return PreparedCorrectAssignmentEvent{}, httperr.NewBadRequest("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return PreparedCorrectAssignmentEvent{}, httperr.NewBadRequest("invalid target_effective_date")
	}

	canonicalPayload, err := CanonicalizeJSONObjectRaw(replacementPayload)
	if err != nil {
		return PreparedCorrectAssignmentEvent{}, err
	}

	return PreparedCorrectAssignmentEvent{
		AssignmentUUID:      assignmentUUID,
		TargetEffectiveDate: targetEffectiveDate,
		CanonicalPayload:    canonicalPayload,
		EventID:             DeterministicAssignmentCorrectionEventID(tenantID, assignmentUUID, targetEffectiveDate, canonicalPayload),
	}, nil
}

func PrepareRescindAssignmentEvent(tenantID string, assignmentUUID string, targetEffectiveDate string, payload json.RawMessage) (PreparedRescindAssignmentEvent, error) {
	assignmentUUID = strings.TrimSpace(assignmentUUID)
	if assignmentUUID == "" {
		return PreparedRescindAssignmentEvent{}, httperr.NewBadRequest("assignment_uuid is required")
	}
	targetEffectiveDate = strings.TrimSpace(targetEffectiveDate)
	if targetEffectiveDate == "" {
		return PreparedRescindAssignmentEvent{}, httperr.NewBadRequest("target_effective_date is required")
	}
	if _, err := time.Parse("2006-01-02", targetEffectiveDate); err != nil {
		return PreparedRescindAssignmentEvent{}, httperr.NewBadRequest("invalid target_effective_date")
	}

	canonicalPayload, err := CanonicalizeJSONObjectOrEmpty(payload)
	if err != nil {
		return PreparedRescindAssignmentEvent{}, err
	}

	return PreparedRescindAssignmentEvent{
		AssignmentUUID:      assignmentUUID,
		TargetEffectiveDate: targetEffectiveDate,
		CanonicalPayload:    canonicalPayload,
		EventID:             DeterministicAssignmentRescindEventID(tenantID, assignmentUUID, targetEffectiveDate),
	}, nil
}

func DeterministicAssignmentCorrectionEventID(tenantID string, assignmentID string, targetEffectiveDate string, canonicalReplacementPayload []byte) string {
	sum := sha256.Sum256(canonicalReplacementPayload)
	name := fmt.Sprintf("staffing.assignment_event_correction:%s:%s:%s:%x", tenantID, assignmentID, targetEffectiveDate, sum[:])
	return uuid.NewSHA1(assignmentCorrectionNamespace, []byte(name)).String()
}

func DeterministicAssignmentRescindEventID(tenantID string, assignmentID string, targetEffectiveDate string) string {
	name := fmt.Sprintf("staffing.assignment_event_rescind:%s:%s:%s", tenantID, assignmentID, targetEffectiveDate)
	return uuid.NewSHA1(assignmentRescindNamespace, []byte(name)).String()
}

func CanonicalizeJSONObjectRaw(raw json.RawMessage) ([]byte, error) {
	raw = json.RawMessage(strings.TrimSpace(string(raw)))
	if len(raw) == 0 {
		return nil, httperr.NewBadRequest("json object is required")
	}

	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, httperr.NewBadRequest("invalid json")
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, httperr.NewBadRequest("json object is required")
	}

	var b strings.Builder
	_ = canonicalizeJSON(&b, v)
	return []byte(b.String()), nil
}

func CanonicalizeJSONObjectOrEmpty(raw json.RawMessage) ([]byte, error) {
	if len(strings.TrimSpace(string(raw))) == 0 || string(raw) == "null" {
		return []byte(`{}`), nil
	}
	return CanonicalizeJSONObjectRaw(raw)
}

func canonicalizeJSON(b *strings.Builder, v any) error {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sortStrings(keys)
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			ks, _ := json.Marshal(k)
			b.Write(ks)
			b.WriteByte(':')
			if err := canonicalizeJSON(b, t[k]); err != nil {
				return err
			}
		}
		b.WriteByte('}')
		return nil
	case []any:
		b.WriteByte('[')
		for i := range t {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := canonicalizeJSON(b, t[i]); err != nil {
				return err
			}
		}
		b.WriteByte(']')
		return nil
	case json.Number:
		b.WriteString(t.String())
		return nil
	case string, bool, nil:
		bb, _ := json.Marshal(t)
		b.Write(bb)
		return nil
	default:
		bb, err := json.Marshal(t)
		if err != nil {
			return err
		}
		b.Write(bb)
		return nil
	}
}

func sortStrings(ss []string) {
	for i := range ss {
		for j := i + 1; j < len(ss); j++ {
			if ss[j] < ss[i] {
				ss[i], ss[j] = ss[j], ss[i]
			}
		}
	}
}

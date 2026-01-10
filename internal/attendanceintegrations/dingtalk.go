package attendanceintegrations

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type dingTalkAttendanceCheckRecordPayload struct {
	DataList []dingTalkAttendanceCheckRecord `json:"dataList"`
	Data     struct {
		DataList []dingTalkAttendanceCheckRecord `json:"dataList"`
	} `json:"data"`
}

type dingTalkAttendanceCheckRecord struct {
	UserID      string `json:"userId"`
	CheckTimeMs int64  `json:"checkTime"`
	BizID       string `json:"bizId"`
	CheckByUser bool   `json:"checkByUser"`
}

func BuildDingTalkAttendanceCheckRecordPunches(eventID string, corpID string, rawPayload []byte) ([]ExternalPunch, error) {
	eventID = strings.TrimSpace(eventID)
	if eventID == "" {
		return nil, errors.New("event_id is required")
	}
	corpID = strings.TrimSpace(corpID)
	if corpID == "" {
		return nil, errors.New("corp_id is required")
	}

	var p dingTalkAttendanceCheckRecordPayload
	if err := json.Unmarshal(rawPayload, &p); err != nil {
		return nil, err
	}

	records := p.DataList
	if len(records) == 0 {
		records = p.Data.DataList
	}
	if len(records) == 0 {
		return nil, errors.New("dataList is required")
	}

	out := make([]ExternalPunch, 0, len(records))
	for _, r := range records {
		userID := strings.TrimSpace(r.UserID)
		if userID == "" {
			return nil, errors.New("userId is required")
		}
		if r.CheckTimeMs <= 0 {
			return nil, errors.New("checkTime must be > 0")
		}
		bizID := strings.TrimSpace(r.BizID)
		if bizID == "" {
			return nil, errors.New("bizId is required")
		}

		requestID := "dingtalk:attendance_check_record:" + eventID + ":" + bizID

		payload := map[string]any{
			"source_provider":   string(ProviderDingTalk),
			"source_event_type": "attendance_check_record",
			"source_event_id":   eventID,
			"external_user_id":  userID,
		}
		payloadJSON, _ := json.Marshal(payload)

		raw := map[string]any{
			"event_type":    "attendance_check_record",
			"event_id":      eventID,
			"corp_id":       corpID,
			"user_id":       userID,
			"check_time_ms": r.CheckTimeMs,
			"biz_id":        bizID,
			"check_by_user": r.CheckByUser,
		}
		rawJSON, _ := json.Marshal(raw)

		out = append(out, ExternalPunch{
			Provider:         ProviderDingTalk,
			ExternalUserID:   userID,
			PunchTime:        time.UnixMilli(r.CheckTimeMs).UTC(),
			PunchType:        "RAW",
			RequestID:        requestID,
			Payload:          payloadJSON,
			SourceRawPayload: rawJSON,
			DeviceInfo:       json.RawMessage(`{}`),
			LastSeenPayload:  rawJSON,
		})
	}
	return out, nil
}

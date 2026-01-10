package attendanceintegrations

import (
	"encoding/json"
	"testing"
)

func TestBuildDingTalkAttendanceCheckRecordPunches(t *testing.T) {
	t.Run("event_id missing", func(t *testing.T) {
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("", "corp1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("corp_id missing", func(t *testing.T) {
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", []byte(`{`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("missing dataList", func(t *testing.T) {
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", []byte(`{}`)); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("nested data.dataList", func(t *testing.T) {
		raw := []byte(`{"data":{"dataList":[{"userId":"u1","checkTime":1570791880000,"bizId":"b1","checkByUser":true}]}}`)
		punches, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(punches) != 1 {
			t.Fatalf("expected 1, got %d", len(punches))
		}
		if punches[0].Provider != ProviderDingTalk || punches[0].ExternalUserID != "u1" || punches[0].PunchType != "RAW" {
			t.Fatalf("punch=%+v", punches[0])
		}
	})

	t.Run("record userId missing", func(t *testing.T) {
		raw := []byte(`{"dataList":[{"userId":" ","checkTime":1,"bizId":"b1"}]}`)
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", raw); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("record checkTime invalid", func(t *testing.T) {
		raw := []byte(`{"dataList":[{"userId":"u1","checkTime":0,"bizId":"b1"}]}`)
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", raw); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("record bizId missing", func(t *testing.T) {
		raw := []byte(`{"dataList":[{"userId":"u1","checkTime":1,"bizId":" "}]}`)
		if _, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", raw); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		raw := []byte(`{"dataList":[{"userId":"u1","checkTime":1570791880000,"bizId":"b1","checkByUser":true}]}`)
		punches, err := BuildDingTalkAttendanceCheckRecordPunches("e1", "corp1", raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(punches) != 1 {
			t.Fatalf("expected 1, got %d", len(punches))
		}
		p := punches[0]
		if p.RequestID != "dingtalk:attendance_check_record:e1:b1" {
			t.Fatalf("request_id=%q", p.RequestID)
		}

		var payload map[string]any
		if !json.Valid(p.Payload) || json.Unmarshal(p.Payload, &payload) != nil {
			t.Fatalf("payload=%q", string(p.Payload))
		}
		if payload["source_provider"] != string(ProviderDingTalk) || payload["external_user_id"] != "u1" || payload["source_event_id"] != "e1" {
			t.Fatalf("payload=%v", payload)
		}

		var rawObj map[string]any
		if !json.Valid(p.SourceRawPayload) || json.Unmarshal(p.SourceRawPayload, &rawObj) != nil {
			t.Fatalf("raw=%q", string(p.SourceRawPayload))
		}
		if rawObj["corp_id"] != "corp1" || rawObj["event_id"] != "e1" || rawObj["biz_id"] != "b1" {
			t.Fatalf("raw=%v", rawObj)
		}
	})
}

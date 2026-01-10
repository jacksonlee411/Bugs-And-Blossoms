package attendanceintegrations

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestWeComAPIError(t *testing.T) {
	if got := (WeComAPIError{Code: 400}).Error(); got != "wecom api error: errcode=400" {
		t.Fatalf("got=%q", got)
	}
	if got := (WeComAPIError{Code: 400, Msg: "bad"}).Error(); got != "wecom api error: errcode=400 errmsg=bad" {
		t.Fatalf("got=%q", got)
	}
}

func TestNewWeComClient(t *testing.T) {
	c1 := NewWeComClient("cid", "sec", nil)
	if c1.HTTPClient == nil {
		t.Fatal("expected non-nil http client")
	}
	hc := &http.Client{}
	c2 := NewWeComClient("cid", "sec", hc)
	if c2.HTTPClient != hc {
		t.Fatal("expected provided http client")
	}
}

func TestWeComClient_GetAccessToken(t *testing.T) {
	t.Run("corpid missing", func(t *testing.T) {
		c := NewWeComClient("", "s", http.DefaultClient)
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("corpsecret missing", func(t *testing.T) {
		c := NewWeComClient("c", "", http.DefaultClient)
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("base url missing", func(t *testing.T) {
		c := NewWeComClient("c", "s", http.DefaultClient)
		c.BaseURL = ""
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("url parse error", func(t *testing.T) {
		c := NewWeComClient("c", "s", http.DefaultClient)
		c.BaseURL = "http://[::1"
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("http do error", func(t *testing.T) {
		c := NewWeComClient("c", "s", &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("do") }),
		})
		c.BaseURL = "https://example.invalid"
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("http status not ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("nope"))
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(wecomTokenResponse{ErrCode: 40013, ErrMsg: "bad"})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(wecomTokenResponse{ErrCode: 0, ErrMsg: "ok", AccessToken: "", ExpiresIn: 7200})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid expires_in", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(wecomTokenResponse{ErrCode: 0, ErrMsg: "ok", AccessToken: "t1", ExpiresIn: 0})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		_, _, err := c.GetAccessToken(context.Background())
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/cgi-bin/gettoken" {
				t.Fatalf("path=%q", r.URL.Path)
			}
			if r.URL.Query().Get("corpid") != "c" || r.URL.Query().Get("corpsecret") != "s" {
				t.Fatalf("query=%v", r.URL.Query())
			}
			_ = json.NewEncoder(w).Encode(wecomTokenResponse{ErrCode: 0, ErrMsg: "ok", AccessToken: "t1", ExpiresIn: 7200})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		token, exp, err := c.GetAccessToken(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if token != "t1" || exp != 7200 {
			t.Fatalf("token=%q exp=%d", token, exp)
		}
	})
}

func TestWeComTokenSource(t *testing.T) {
	t.Run("cached token", func(t *testing.T) {
		now := time.Unix(100, 0).UTC()
		s := &WeComTokenSource{
			client:    nil,
			now:       func() time.Time { return now },
			token:     "cached",
			expiresAt: now.Add(10 * time.Minute),
		}
		if got, err := s.Token(context.Background()); err != nil || got != "cached" {
			t.Fatalf("got=%q err=%v", got, err)
		}
	})

	t.Run("refresh token", func(t *testing.T) {
		now := time.Unix(100, 0).UTC()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(wecomTokenResponse{ErrCode: 0, ErrMsg: "ok", AccessToken: "new", ExpiresIn: 60})
		}))
		t.Cleanup(srv.Close)

		client := NewWeComClient("c", "s", srv.Client())
		client.BaseURL = srv.URL
		ts := NewWeComTokenSource(client)
		ts.now = func() time.Time { return now }

		got, err := ts.Token(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got != "new" || !ts.expiresAt.Equal(now.Add(60*time.Second)) {
			t.Fatalf("got=%q expiresAt=%v", got, ts.expiresAt)
		}

		ts.Invalidate()
		if ts.token != "" || !ts.expiresAt.IsZero() {
			t.Fatalf("token=%q expiresAt=%v", ts.token, ts.expiresAt)
		}
	})

	t.Run("refresh error", func(t *testing.T) {
		ts := NewWeComTokenSource(NewWeComClient("", "s", http.DefaultClient))
		if _, err := ts.Token(context.Background()); err == nil {
			t.Fatal("expected error")
		}
	})
}

func TestWeComClient_GetCheckinData(t *testing.T) {
	t.Run("token missing", func(t *testing.T) {
		if _, err := NewWeComClient("c", "s", http.DefaultClient).GetCheckinData(context.Background(), "", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("invalid time", func(t *testing.T) {
		if _, err := NewWeComClient("c", "s", http.DefaultClient).GetCheckinData(context.Background(), "t", 2, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("useridlist empty after trim", func(t *testing.T) {
		if _, err := NewWeComClient("c", "s", http.DefaultClient).GetCheckinData(context.Background(), "t", 1, 1, []string{" ", ""}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("base url missing", func(t *testing.T) {
		c := NewWeComClient("c", "s", http.DefaultClient)
		c.BaseURL = ""
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("url parse error", func(t *testing.T) {
		c := NewWeComClient("c", "s", http.DefaultClient)
		c.BaseURL = "http://[::1"
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("http do error", func(t *testing.T) {
		c := NewWeComClient("c", "s", &http.Client{
			Transport: roundTripperFunc(func(*http.Request) (*http.Response, error) { return nil, errors.New("do") }),
		})
		c.BaseURL = "https://example.invalid"
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("http status not ok", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("nope"))
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("decode error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("not-json"))
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("api error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_ = json.NewEncoder(w).Encode(wecomCheckinResponse{ErrCode: 40014, ErrMsg: "invalid token"})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		if _, err := c.GetCheckinData(context.Background(), "t", 1, 1, []string{"u"}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/cgi-bin/checkin/getcheckindata" {
				t.Fatalf("path=%q", r.URL.Path)
			}
			if r.URL.Query().Get("access_token") != "t" {
				t.Fatalf("query=%v", r.URL.Query())
			}
			var body wecomCheckinRequest
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.OpenCheckinDataType != 3 || body.StartTime != 1 || body.EndTime != 2 || len(body.UserIDList) != 1 || body.UserIDList[0] != "u" {
				t.Fatalf("body=%+v", body)
			}
			_ = json.NewEncoder(w).Encode(wecomCheckinResponse{
				ErrCode: 0,
				ErrMsg:  "ok",
				CheckinData: []WeComCheckinRecord{
					{UserID: "u", CheckinTime: 1, CheckinType: "上班打卡"},
				},
			})
		}))
		t.Cleanup(srv.Close)

		c := NewWeComClient("c", "s", srv.Client())
		c.BaseURL = srv.URL
		got, err := c.GetCheckinData(context.Background(), "t", 1, 2, []string{" u ", ""})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 || got[0].UserID != "u" {
			t.Fatalf("got=%+v", got)
		}
	})
}

func TestBuildWeComCheckinPunches(t *testing.T) {
	t.Run("userid missing", func(t *testing.T) {
		if _, err := BuildWeComCheckinPunches([]WeComCheckinRecord{{}}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("checkin_time invalid", func(t *testing.T) {
		if _, err := BuildWeComCheckinPunches([]WeComCheckinRecord{{UserID: "u", CheckinType: "上班打卡"}}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("checkin_type missing", func(t *testing.T) {
		if _, err := BuildWeComCheckinPunches([]WeComCheckinRecord{{UserID: "u", CheckinTime: 1}}); err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("success mapping", func(t *testing.T) {
		checkins := []WeComCheckinRecord{
			{UserID: "u1", CheckinTime: 1, CheckinType: "上班打卡", ExceptionType: "E"},
			{UserID: "u2", CheckinTime: 2, CheckinType: "下班打卡"},
			{UserID: "u3", CheckinTime: 3, CheckinType: "外出"},
		}
		punches, err := BuildWeComCheckinPunches(checkins)
		if err != nil {
			t.Fatal(err)
		}
		if len(punches) != 3 {
			t.Fatalf("expected 3, got %d", len(punches))
		}
		if punches[0].PunchType != "IN" || punches[1].PunchType != "OUT" || punches[2].PunchType != "RAW" {
			t.Fatalf("punches=%+v", punches)
		}
		if !strings.Contains(string(punches[0].Payload), "exception_type") || !strings.Contains(string(punches[0].SourceRawPayload), "exception_type") {
			t.Fatalf("payload=%q raw=%q", string(punches[0].Payload), string(punches[0].SourceRawPayload))
		}
		if strings.Contains(string(punches[1].Payload), "exception_type") || strings.Contains(string(punches[1].SourceRawPayload), "exception_type") {
			t.Fatalf("payload=%q raw=%q", string(punches[1].Payload), string(punches[1].SourceRawPayload))
		}
	})
}

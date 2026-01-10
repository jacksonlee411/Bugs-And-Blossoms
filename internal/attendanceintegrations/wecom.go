package attendanceintegrations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type WeComAPIError struct {
	Code int
	Msg  string
}

func (e WeComAPIError) Error() string {
	if e.Msg == "" {
		return fmt.Sprintf("wecom api error: errcode=%d", e.Code)
	}
	return fmt.Sprintf("wecom api error: errcode=%d errmsg=%s", e.Code, e.Msg)
}

type WeComClient struct {
	BaseURL    string
	CorpID     string
	CorpSecret string
	HTTPClient *http.Client
}

func NewWeComClient(corpID string, corpSecret string, httpClient *http.Client) *WeComClient {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &WeComClient{
		BaseURL:    "https://qyapi.weixin.qq.com",
		CorpID:     corpID,
		CorpSecret: corpSecret,
		HTTPClient: httpClient,
	}
}

type wecomTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int64  `json:"expires_in"`
}

func (c *WeComClient) GetAccessToken(ctx context.Context) (token string, expiresInSeconds int64, _ error) {
	corpID := strings.TrimSpace(c.CorpID)
	if corpID == "" {
		return "", 0, errors.New("wecom corpid is required")
	}
	secret := strings.TrimSpace(c.CorpSecret)
	if secret == "" {
		return "", 0, errors.New("wecom corpsecret is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return "", 0, errors.New("wecom base_url is required")
	}

	u, err := url.Parse(baseURL + "/cgi-bin/gettoken")
	if err != nil {
		return "", 0, err
	}
	q := u.Query()
	q.Set("corpid", corpID)
	q.Set("corpsecret", secret)
	u.RawQuery = q.Encode()

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("wecom gettoken http status=%d body=%q", resp.StatusCode, string(body))
	}

	var tr wecomTokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tr); err != nil {
		return "", 0, err
	}
	if tr.ErrCode != 0 {
		return "", 0, WeComAPIError{Code: tr.ErrCode, Msg: tr.ErrMsg}
	}
	if strings.TrimSpace(tr.AccessToken) == "" {
		return "", 0, errors.New("wecom gettoken returned empty access_token")
	}
	if tr.ExpiresIn <= 0 {
		return "", 0, errors.New("wecom gettoken returned invalid expires_in")
	}
	return tr.AccessToken, tr.ExpiresIn, nil
}

type WeComTokenSource struct {
	client *WeComClient
	now    func() time.Time

	token     string
	expiresAt time.Time
}

func NewWeComTokenSource(client *WeComClient) *WeComTokenSource {
	return &WeComTokenSource{client: client, now: time.Now}
}

func (s *WeComTokenSource) Invalidate() {
	s.token = ""
	s.expiresAt = time.Time{}
}

func (s *WeComTokenSource) Token(ctx context.Context) (string, error) {
	if s.token != "" && !s.expiresAt.IsZero() && s.now().Before(s.expiresAt.Add(-30*time.Second)) {
		return s.token, nil
	}

	token, expiresInSeconds, err := s.client.GetAccessToken(ctx)
	if err != nil {
		return "", err
	}

	s.token = token
	s.expiresAt = s.now().Add(time.Duration(expiresInSeconds) * time.Second)
	return s.token, nil
}

type wecomCheckinRequest struct {
	OpenCheckinDataType int      `json:"opencheckindatatype"`
	StartTime           int64    `json:"starttime"`
	EndTime             int64    `json:"endtime"`
	UserIDList          []string `json:"useridlist"`
}

type WeComCheckinRecord struct {
	UserID        string `json:"userid"`
	CheckinTime   int64  `json:"checkin_time"`
	CheckinType   string `json:"checkin_type"`
	ExceptionType string `json:"exception_type"`
}

type wecomCheckinResponse struct {
	ErrCode     int                  `json:"errcode"`
	ErrMsg      string               `json:"errmsg"`
	CheckinData []WeComCheckinRecord `json:"checkindata"`
}

func (c *WeComClient) GetCheckinData(ctx context.Context, accessToken string, startTime int64, endTime int64, userIDs []string) ([]WeComCheckinRecord, error) {
	accessToken = strings.TrimSpace(accessToken)
	if accessToken == "" {
		return nil, errors.New("wecom access_token is required")
	}
	if startTime <= 0 || endTime <= 0 || startTime > endTime {
		return nil, errors.New("wecom invalid start/end time")
	}

	var cleaned []string
	for _, u := range userIDs {
		u = strings.TrimSpace(u)
		if u != "" {
			cleaned = append(cleaned, u)
		}
	}
	if len(cleaned) == 0 {
		return nil, errors.New("wecom useridlist is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return nil, errors.New("wecom base_url is required")
	}
	u, err := url.Parse(baseURL + "/cgi-bin/checkin/getcheckindata")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("access_token", accessToken)
	u.RawQuery = q.Encode()

	reqBody, _ := json.Marshal(wecomCheckinRequest{
		OpenCheckinDataType: 3,
		StartTime:           startTime,
		EndTime:             endTime,
		UserIDList:          cleaned,
	})
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("wecom getcheckindata http status=%d body=%q", resp.StatusCode, string(body))
	}

	var cr wecomCheckinResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return nil, err
	}
	if cr.ErrCode != 0 {
		return nil, WeComAPIError{Code: cr.ErrCode, Msg: cr.ErrMsg}
	}
	return cr.CheckinData, nil
}

func BuildWeComCheckinPunches(checkins []WeComCheckinRecord) ([]ExternalPunch, error) {
	out := make([]ExternalPunch, 0, len(checkins))
	for _, c := range checkins {
		userID := strings.TrimSpace(c.UserID)
		if userID == "" {
			return nil, errors.New("userid is required")
		}
		if c.CheckinTime <= 0 {
			return nil, errors.New("checkin_time must be > 0")
		}
		checkinType := strings.TrimSpace(c.CheckinType)
		if checkinType == "" {
			return nil, errors.New("checkin_type is required")
		}
		exceptionType := strings.TrimSpace(c.ExceptionType)

		punchType := "RAW"
		if checkinType == "上班打卡" {
			punchType = "IN"
		} else if checkinType == "下班打卡" {
			punchType = "OUT"
		}

		requestID := "wecom:checkin:" + userID + ":" + strconv.FormatInt(c.CheckinTime, 10) + ":" + checkinType

		payload := map[string]any{
			"source_provider":   string(ProviderWeCom),
			"source_event_type": "checkin",
			"external_user_id":  userID,
			"checkin_type":      checkinType,
		}
		if exceptionType != "" {
			payload["exception_type"] = exceptionType
		}
		payloadJSON, _ := json.Marshal(payload)

		raw := map[string]any{
			"source":       "wecom",
			"userid":       userID,
			"checkin_time": c.CheckinTime,
			"checkin_type": checkinType,
		}
		if exceptionType != "" {
			raw["exception_type"] = exceptionType
		}
		rawJSON, _ := json.Marshal(raw)

		out = append(out, ExternalPunch{
			Provider:         ProviderWeCom,
			ExternalUserID:   userID,
			PunchTime:        time.Unix(c.CheckinTime, 0).UTC(),
			PunchType:        punchType,
			RequestID:        requestID,
			Payload:          payloadJSON,
			SourceRawPayload: rawJSON,
			DeviceInfo:       json.RawMessage(`{}`),
			LastSeenPayload:  rawJSON,
		})
	}
	return out, nil
}

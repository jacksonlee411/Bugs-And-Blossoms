package kratos

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Client struct {
	publicBaseURL string
	httpClient    *http.Client
}

type Identity struct {
	ID     string         `json:"id"`
	Traits map[string]any `json:"traits"`
	Raw    map[string]any `json:"-"`
}

type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	msg := strings.TrimSpace(e.Message)
	if msg == "" {
		msg = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("kratos: http %d: %s", e.StatusCode, msg)
}

func New(publicBaseURL string) (*Client, error) {
	publicBaseURL = strings.TrimSpace(publicBaseURL)
	publicBaseURL = strings.TrimRight(publicBaseURL, "/")
	if publicBaseURL == "" {
		return nil, errors.New("kratos: missing public base url")
	}
	u, err := url.Parse(publicBaseURL)
	if err != nil {
		return nil, errors.New("kratos: invalid public base url")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("kratos: invalid public base url scheme")
	}
	if u.Host == "" {
		return nil, errors.New("kratos: invalid public base url host")
	}
	return &Client{
		publicBaseURL: publicBaseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (c *Client) LoginPassword(ctx context.Context, identifier string, password string) (Identity, error) {
	flowID, err := c.createLoginFlow(ctx)
	if err != nil {
		return Identity{}, err
	}
	sessionToken, err := c.submitLoginPassword(ctx, flowID, identifier, password)
	if err != nil {
		return Identity{}, err
	}
	return c.Whoami(ctx, sessionToken)
}

func (c *Client) Whoami(ctx context.Context, sessionToken string) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.publicBaseURL+"/sessions/whoami", nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Session-Token", sessionToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return Identity{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return Identity{}, readHTTPError(resp)
	}

	var out struct {
		Identity Identity `json:"identity"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Identity{}, err
	}
	return out.Identity, nil
}

func (c *Client) createLoginFlow(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.publicBaseURL+"/self-service/login/api", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", readHTTPError(resp)
	}

	var out struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.ID == "" {
		return "", errors.New("kratos: missing login flow id")
	}
	return out.ID, nil
}

func (c *Client) submitLoginPassword(ctx context.Context, flowID string, identifier string, password string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"method":     "password",
		"identifier": identifier,
		"password":   password,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.publicBaseURL+"/self-service/login?flow="+flowID, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return "", readHTTPError(resp)
	}

	var out struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if out.SessionToken == "" {
		return "", errors.New("kratos: missing session token")
	}
	return out.SessionToken, nil
}

func readHTTPError(resp *http.Response) error {
	const maxBody = 4096
	b, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	return &HTTPError{
		StatusCode: resp.StatusCode,
		Message:    string(b),
	}
}

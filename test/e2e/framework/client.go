package framework

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/fatedier/frp/client/http/model"
	"github.com/fatedier/frp/test/e2e/framework/consts"
)

// E2EAdminClient logs in with the session-authenticated admin API and keeps
// its cookie jar and CSRF token for subsequent management requests.
type E2EAdminClient struct {
	address string
	client  *http.Client

	loginMu sync.Mutex
	csrf    string
}

func (f *Framework) APIClientForFrpc(port int) *E2EAdminClient {
	jar, _ := cookiejar.New(nil)
	return &E2EAdminClient{
		address: net.JoinHostPort("127.0.0.1", strconv.Itoa(port)),
		client:  &http.Client{Jar: jar},
	}
}

func (c *E2EAdminClient) GetProxyStatus(ctx context.Context, name string) (*model.ProxyStatusResp, error) {
	allStatus, err := c.GetAllProxyStatus(ctx)
	if err != nil {
		return nil, err
	}
	for _, proxies := range allStatus {
		for _, proxy := range proxies {
			if proxy.Name == name {
				return &proxy, nil
			}
		}
	}
	return nil, fmt.Errorf("no proxy status found")
}

func (c *E2EAdminClient) GetAllProxyStatus(ctx context.Context) (model.StatusResp, error) {
	content, err := c.do(ctx, http.MethodGet, "/api/status", nil)
	if err != nil {
		return nil, err
	}
	status := make(model.StatusResp)
	if err := json.Unmarshal([]byte(content), &status); err != nil {
		return nil, err
	}
	return status, nil
}

func (c *E2EAdminClient) Reload(ctx context.Context, strict bool) error {
	query := url.Values{}
	if strict {
		query.Set("strictConfig", "true")
	}
	path := "/api/reload"
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	_, err := c.do(ctx, http.MethodGet, path, nil)
	return err
}

func (c *E2EAdminClient) Stop(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodPost, "/api/stop", nil)
	return err
}

func (c *E2EAdminClient) GetConfig(ctx context.Context) (string, error) {
	return c.do(ctx, http.MethodGet, "/api/config", nil)
}

func (c *E2EAdminClient) UpdateConfig(ctx context.Context, content string) error {
	_, err := c.do(ctx, http.MethodPut, "/api/config", strings.NewReader(content))
	return err
}

// Do performs an authenticated management request and returns the raw status
// and body so tests can assert expected error responses.
func (c *E2EAdminClient) Do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	if err := c.ensureLogin(ctx); err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://"+c.address+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	if mutates(method) {
		req.Header.Set("X-CSRF-Token", c.csrf)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, content, nil
}

func (c *E2EAdminClient) do(ctx context.Context, method, path string, body io.Reader) (string, error) {
	var payload []byte
	var err error
	if body != nil {
		payload, err = io.ReadAll(body)
		if err != nil {
			return "", err
		}
	}
	status, content, err := c.Do(ctx, method, path, payload)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("api status code [%d]: %s", status, strings.TrimSpace(string(content)))
	}
	return string(content), nil
}

func (c *E2EAdminClient) ensureLogin(ctx context.Context) error {
	c.loginMu.Lock()
	defer c.loginMu.Unlock()
	if c.csrf != "" {
		return nil
	}
	payload, err := json.Marshal(map[string]string{
		"username": consts.AdminUsername,
		"password": consts.AdminPassword,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://"+c.address+"/api/v2/auth/login", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("admin login failed [%d]: %s", resp.StatusCode, strings.TrimSpace(string(content)))
	}
	var envelope struct {
		Data struct {
			CSRFToken string `json:"csrfToken"`
		} `json:"data"`
	}
	if err := json.Unmarshal(content, &envelope); err != nil {
		return err
	}
	if envelope.Data.CSRFToken == "" {
		return fmt.Errorf("admin login response did not contain a CSRF token")
	}
	c.csrf = envelope.Data.CSRFToken
	return nil
}

func mutates(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

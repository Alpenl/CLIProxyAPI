package replenishment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

func NewClient(baseURL string, secret string, httpClient *http.Client) (*Client, error) {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{
		baseURL:    baseURL,
		secret:     strings.TrimSpace(secret),
		httpClient: httpClient,
	}, nil
}

func (c *Client) CreateJob(ctx context.Context, input CreateJobInput) (*Job, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	if input.RequestedSuccesses <= 0 {
		return nil, fmt.Errorf("requested successes must be >= 1")
	}

	body, err := json.Marshal(createJobRequest{
		RequestedSuccesses: input.RequestedSuccesses,
		Source:             strings.TrimSpace(input.Source),
		ZipRequired:        input.ZipRequired,
		Callback:           normalizeCallbackConfig(input.Callback),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal create job request: %w", err)
	}

	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send create job request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("create job failed with status %d", resp.StatusCode)
	}

	var payload struct {
		Job *Job `json:"job"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode create job response: %w", err)
	}
	if payload.Job == nil {
		return nil, fmt.Errorf("create job response missing job")
	}
	return payload.Job, nil
}

func normalizeCallbackConfig(input *CallbackConfig) *CallbackConfig {
	if input == nil {
		return nil
	}

	url := strings.TrimSpace(input.URL)
	token := strings.TrimSpace(input.Token)
	if url == "" || token == "" {
		return nil
	}

	return &CallbackConfig{
		URL:   url,
		Token: token,
	}
}

func (c *Client) GetJob(ctx context.Context, id string) (*Job, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job ID is required")
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/jobs/"+id, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send get job request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("get job failed with status %d", resp.StatusCode)
	}

	var payload struct {
		Job *Job `json:"job"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode get job response: %w", err)
	}
	return payload.Job, nil
}

func (c *Client) DownloadArchive(ctx context.Context, id string) ([]byte, error) {
	if c == nil {
		return nil, fmt.Errorf("client is nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, fmt.Errorf("job ID is required")
	}

	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/jobs/"+id+"/archive", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send download archive request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("download archive failed with status %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read archive response: %w", err)
	}
	return data, nil
}

func (c *Client) newRequest(ctx context.Context, method string, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if c.secret != "" {
		req.Header.Set("X-Management-Secret", c.secret)
	}
	return req, nil
}

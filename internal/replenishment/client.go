package replenishment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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
	})
	if err != nil {
		return nil, fmt.Errorf("marshal create job request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/jobs", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.secret != "" {
		req.Header.Set("X-Management-Secret", c.secret)
	}

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

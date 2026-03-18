package replenishment

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientCreateJob_SendsAuthAndPayload(t *testing.T) {
	var capturedAuth string
	var capturedPayload createJobRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("X-Management-Secret")
		if err := json.NewDecoder(r.Body).Decode(&capturedPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job": map[string]any{
				"id":                 "job-1",
				"status":             "queued",
				"requestedSuccesses": 3,
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "service-secret", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	job, err := client.CreateJob(context.Background(), CreateJobInput{
		RequestedSuccesses: 3,
		Source:             "auto",
		ZipRequired:        true,
		Callback: &CallbackConfig{
			URL:   "http://127.0.0.1:9090/v0/internal/replenishment/accounts",
			Token: "callback-secret",
		},
	})
	if err != nil {
		t.Fatalf("CreateJob() error = %v", err)
	}

	if capturedAuth != "service-secret" {
		t.Fatalf("auth header = %q, want service-secret", capturedAuth)
	}
	if capturedPayload.RequestedSuccesses != 3 {
		t.Fatalf("requestedSuccesses = %d, want 3", capturedPayload.RequestedSuccesses)
	}
	if capturedPayload.Callback == nil {
		t.Fatalf("callback = nil, want value")
	}
	if capturedPayload.Callback.URL != "http://127.0.0.1:9090/v0/internal/replenishment/accounts" {
		t.Fatalf("callback.url = %q, want callback url", capturedPayload.Callback.URL)
	}
	if capturedPayload.Callback.Token != "callback-secret" {
		t.Fatalf("callback.token = %q, want callback-secret", capturedPayload.Callback.Token)
	}
	if job.ID != "job-1" {
		t.Fatalf("job.ID = %q, want job-1", job.ID)
	}
}

func TestClientGetJob_SendsAuthAndParsesResponse(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs/job-1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("X-Management-Secret")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job": map[string]any{
				"id":                 "job-1",
				"status":             "running",
				"requestedSuccesses": 2,
				"successCount":       1,
			},
		})
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "service-secret", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	job, err := client.GetJob(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}

	if capturedAuth != "service-secret" {
		t.Fatalf("auth header = %q, want service-secret", capturedAuth)
	}
	if job == nil {
		t.Fatalf("job = nil, want value")
	}
	if job.Status != "running" {
		t.Fatalf("job.Status = %q, want running", job.Status)
	}
	if job.SuccessCount != 1 {
		t.Fatalf("job.SuccessCount = %d, want 1", job.SuccessCount)
	}
}

func TestClientGetJob_NotFoundReturnsNil(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "service-secret", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	job, err := client.GetJob(context.Background(), "missing")
	if err != nil {
		t.Fatalf("GetJob() error = %v", err)
	}
	if job != nil {
		t.Fatalf("job = %#v, want nil", job)
	}
}

func TestClientDownloadArchive_ReturnsBytes(t *testing.T) {
	var capturedAuth string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/jobs/job-1/archive" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		capturedAuth = r.Header.Get("X-Management-Secret")
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write([]byte("zip-bytes"))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "service-secret", server.Client())
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	archive, err := client.DownloadArchive(context.Background(), "job-1")
	if err != nil {
		t.Fatalf("DownloadArchive() error = %v", err)
	}

	if capturedAuth != "service-secret" {
		t.Fatalf("auth header = %q, want service-secret", capturedAuth)
	}
	if string(archive) != "zip-bytes" {
		t.Fatalf("archive = %q, want zip-bytes", string(archive))
	}
}

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
	if job.ID != "job-1" {
		t.Fatalf("job.ID = %q, want job-1", job.ID)
	}
}

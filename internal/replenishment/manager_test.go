package replenishment

import (
	"context"
	"errors"
	"testing"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type fakeControlClient struct {
	created []CreateJobInput
	job     *Job
	archive []byte
	downloads int
	err     error
	getJob  func(context.Context, string, *fakeControlClient) (*Job, error)
}

func (c *fakeControlClient) CreateJob(_ context.Context, input CreateJobInput) (*Job, error) {
	c.created = append(c.created, input)
	if c.err != nil {
		return nil, c.err
	}
	if c.job == nil {
		c.job = &Job{
			ID:                 "job-1",
			Status:             "queued",
			RequestedSuccesses: input.RequestedSuccesses,
		}
	}
	return c.job, nil
}

func (c *fakeControlClient) GetJob(ctx context.Context, id string) (*Job, error) {
	if c.getJob != nil {
		return c.getJob(ctx, id, c)
	}
	if c.job != nil && c.job.ID == id {
		return c.job, nil
	}
	return nil, nil
}

func (c *fakeControlClient) DownloadArchive(_ context.Context, id string) ([]byte, error) {
	if c.job != nil && c.job.ID == id {
		c.downloads++
		return c.archive, nil
	}
	return nil, nil
}

func TestComputeDeficit_ExcludesReservedJobs(t *testing.T) {
	snapshot := PoolSnapshot{
		Target:   10,
		Healthy:  6,
		Warming:  1,
		Reserved: 2,
	}

	if got := ComputeDeficit(snapshot); got != 1 {
		t.Fatalf("ComputeDeficit() = %d, want 1", got)
	}
}

func TestSummarizeAuthPool_ClassifiesCodexAccounts(t *testing.T) {
	now := time.Now()
	auths := []*coreauth.Auth{
		{
			Provider: "codex",
			Status:   coreauth.StatusActive,
		},
		{
			Provider: "codex",
			Status:   coreauth.StatusPending,
		},
		{
			Provider:    "codex",
			Status:      coreauth.StatusActive,
			Unavailable: true,
			Quota: coreauth.QuotaState{
				Exceeded:      true,
				NextRecoverAt: now.Add(time.Hour),
			},
		},
		{
			Provider: "codex",
			Status:   coreauth.StatusError,
		},
		{
			Provider: "other",
			Status:   coreauth.StatusActive,
		},
	}

	summary := SummarizeAuthPool(auths)
	if summary.Total != 4 {
		t.Fatalf("Total = %d, want 4", summary.Total)
	}
	if summary.Healthy != 1 {
		t.Fatalf("Healthy = %d, want 1", summary.Healthy)
	}
	if summary.Warming != 1 {
		t.Fatalf("Warming = %d, want 1", summary.Warming)
	}
	if summary.Cooling != 1 {
		t.Fatalf("Cooling = %d, want 1", summary.Cooling)
	}
	if summary.Invalid != 1 {
		t.Fatalf("Invalid = %d, want 1", summary.Invalid)
	}
}

func TestAutoManagerRunOnce_CreatesJobThenImportsArchive(t *testing.T) {
	client := &fakeControlClient{}
	imported := 0
	manager := NewAutoManager(AutoManagerOptions{
		GetConfig: func() ConfigSnapshot {
			return ConfigSnapshot{
				Enabled:            true,
				TargetAccountCount: 3,
			}
		},
		ListAuths: func() []*coreauth.Auth {
			return []*coreauth.Auth{
				{Provider: "codex", Status: coreauth.StatusActive},
			}
		},
		Client: client,
		ImportArchive: func(_ context.Context, data []byte) error {
			if string(data) != "archive-bytes" {
				t.Fatalf("unexpected archive payload: %q", string(data))
			}
			imported++
			return nil
		},
	})

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(create) error = %v", err)
	}
	if len(client.created) != 1 {
		t.Fatalf("created jobs = %d, want 1", len(client.created))
	}
	if client.created[0].RequestedSuccesses != 2 {
		t.Fatalf("requested successes = %d, want 2", client.created[0].RequestedSuccesses)
	}

	client.job.Status = "completed"
	client.archive = []byte("archive-bytes")

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(import) error = %v", err)
	}
	if imported != 1 {
		t.Fatalf("imported = %d, want 1", imported)
	}
	if pending := manager.CurrentJob(); pending != nil {
		t.Fatalf("pending job = %#v, want nil", pending)
	}
}

func TestAutoManagerRunOnce_BacksOffAfterCreateFailure(t *testing.T) {
	now := time.Date(2026, 3, 17, 12, 0, 0, 0, time.UTC)
	client := &fakeControlClient{err: errors.New("service unavailable")}

	manager := NewAutoManager(AutoManagerOptions{
		GetConfig: func() ConfigSnapshot {
			return ConfigSnapshot{
				Enabled:            true,
				TargetAccountCount: 2,
			}
		},
		ListAuths: func() []*coreauth.Auth { return nil },
		Client:    client,
		Now: func() time.Time {
			return now
		},
	})

	if err := manager.RunOnce(context.Background()); err == nil {
		t.Fatalf("RunOnce(first) error = nil, want service unavailable")
	}
	if got := len(client.created); got != 1 {
		t.Fatalf("created jobs after first failure = %d, want 1", got)
	}

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(during backoff) error = %v, want nil", err)
	}
	if got := len(client.created); got != 1 {
		t.Fatalf("created jobs during backoff = %d, want still 1", got)
	}

	now = now.Add(time.Minute)
	if err := manager.RunOnce(context.Background()); err == nil {
		t.Fatalf("RunOnce(after backoff) error = nil, want service unavailable")
	}
	if got := len(client.created); got != 2 {
		t.Fatalf("created jobs after backoff = %d, want 2", got)
	}
}

func TestAutoManagerRunManual_ImportsArchiveSoonAfterJobCompletes(t *testing.T) {
	prevPollInterval := pendingReconcilePollInterval
	prevPollTimeout := pendingReconcileRequestTimeout
	pendingReconcilePollInterval = time.Millisecond
	pendingReconcileRequestTimeout = 50 * time.Millisecond
	defer func() {
		pendingReconcilePollInterval = prevPollInterval
		pendingReconcileRequestTimeout = prevPollTimeout
	}()

	client := &fakeControlClient{
		archive: []byte("archive-bytes"),
	}
	client.getJob = func(_ context.Context, id string, c *fakeControlClient) (*Job, error) {
		if c.job == nil || c.job.ID != id {
			return nil, nil
		}
		completed := *c.job
		completed.Status = "completed"
		return &completed, nil
	}

	imported := make(chan struct{}, 1)
	manager := NewAutoManager(AutoManagerOptions{
		GetConfig: func() ConfigSnapshot {
			return ConfigSnapshot{
				Enabled:            true,
				TargetAccountCount: 2,
			}
		},
		ListAuths: func() []*coreauth.Auth { return nil },
		Client:    client,
		ImportArchive: func(_ context.Context, data []byte) error {
			if string(data) != "archive-bytes" {
				t.Fatalf("unexpected archive payload: %q", string(data))
			}
			select {
			case imported <- struct{}{}:
			default:
			}
			return nil
		},
	})

	if err := manager.RunManual(context.Background()); err != nil {
		t.Fatalf("RunManual() error = %v", err)
	}
	if got := len(client.created); got != 1 {
		t.Fatalf("created jobs = %d, want 1", got)
	}
	select {
	case <-imported:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for background import")
	}
	if pending := manager.CurrentJob(); pending != nil {
		t.Fatalf("pending job = %#v, want nil", pending)
	}
}

func TestAutoManagerRunOnce_SkipsArchiveDownloadWhenStreamedCallbackAlreadyDeliveredAllSuccesses(t *testing.T) {
	client := &fakeControlClient{}
	imported := 0
	manager := NewAutoManager(AutoManagerOptions{
		GetConfig: func() ConfigSnapshot {
			return ConfigSnapshot{
				Enabled:            true,
				TargetAccountCount: 2,
			}
		},
		ListAuths: func() []*coreauth.Auth { return nil },
		Client:    client,
		ImportArchive: func(_ context.Context, data []byte) error {
			imported++
			_ = data
			return nil
		},
		BuildCreateJobInput: func(requestedSuccesses int) CreateJobInput {
			return CreateJobInput{
				RequestedSuccesses: requestedSuccesses,
				Source:             "auto",
				ZipRequired:        true,
				Callback: &CallbackConfig{
					URL:   "http://127.0.0.1:8317/v0/internal/replenishment/accounts",
					Token: "callback-secret",
				},
			}
		},
	})

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(create) error = %v", err)
	}

	manager.NoteStreamedAccount("job-1", "codex-first.json")
	manager.NoteStreamedAccount("job-1", "codex-second.json")

	client.job.Status = "completed"
	client.job.SuccessCount = 2
	client.archive = []byte("archive-bytes")

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(reconcile) error = %v", err)
	}
	if client.downloads != 0 {
		t.Fatalf("archive downloads = %d, want 0", client.downloads)
	}
	if imported != 0 {
		t.Fatalf("imported archives = %d, want 0", imported)
	}
	if pending := manager.CurrentJob(); pending != nil {
		t.Fatalf("pending job = %#v, want nil", pending)
	}
}

func TestAutoManagerRunOnce_DownloadsArchiveWhenStreamedCallbackDeliveredOnlySubset(t *testing.T) {
	client := &fakeControlClient{}
	imported := 0
	manager := NewAutoManager(AutoManagerOptions{
		GetConfig: func() ConfigSnapshot {
			return ConfigSnapshot{
				Enabled:            true,
				TargetAccountCount: 2,
			}
		},
		ListAuths: func() []*coreauth.Auth { return nil },
		Client:    client,
		ImportArchive: func(_ context.Context, data []byte) error {
			if string(data) != "archive-bytes" {
				t.Fatalf("unexpected archive payload: %q", string(data))
			}
			imported++
			return nil
		},
		BuildCreateJobInput: func(requestedSuccesses int) CreateJobInput {
			return CreateJobInput{
				RequestedSuccesses: requestedSuccesses,
				Source:             "auto",
				ZipRequired:        true,
				Callback: &CallbackConfig{
					URL:   "http://127.0.0.1:8317/v0/internal/replenishment/accounts",
					Token: "callback-secret",
				},
			}
		},
	})

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(create) error = %v", err)
	}

	manager.NoteStreamedAccount("job-1", "codex-first.json")

	client.job.Status = "completed"
	client.job.SuccessCount = 2
	client.archive = []byte("archive-bytes")

	if err := manager.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce(reconcile) error = %v", err)
	}
	if client.downloads != 1 {
		t.Fatalf("archive downloads = %d, want 1", client.downloads)
	}
	if imported != 1 {
		t.Fatalf("imported archives = %d, want 1", imported)
	}
}

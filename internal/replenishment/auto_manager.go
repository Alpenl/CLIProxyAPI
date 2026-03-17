package replenishment

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

type ConfigSnapshot struct {
	Enabled            bool
	TargetAccountCount int
}

type JobService interface {
	CreateJob(context.Context, CreateJobInput) (*Job, error)
	GetJob(context.Context, string) (*Job, error)
	DownloadArchive(context.Context, string) ([]byte, error)
}

type AutoManagerOptions struct {
	GetConfig     func() ConfigSnapshot
	ListAuths     func() []*coreauth.Auth
	Client        JobService
	ImportArchive func(context.Context, []byte) error
	Now           func() time.Time
	Backoff       []time.Duration
}

type AutoManager struct {
	getConfig     func() ConfigSnapshot
	listAuths     func() []*coreauth.Auth
	client        JobService
	importArchive func(context.Context, []byte) error
	now           func() time.Time
	backoff       []time.Duration

	mu            sync.Mutex
	pending       *Job
	failureCount  int
	nextAttemptAt time.Time
	watchCancel   context.CancelFunc
	reconcileMu   sync.Mutex
}

var pendingReconcilePollInterval = 2 * time.Second
var pendingReconcileRequestTimeout = 30 * time.Second

type StateSnapshot struct {
	CurrentJob    *Job
	FailureCount  int
	NextAttemptAt time.Time
}

func NewAutoManager(options AutoManagerOptions) *AutoManager {
	manager := &AutoManager{
		client:        options.Client,
		importArchive: options.ImportArchive,
	}
	if options.GetConfig != nil {
		manager.getConfig = options.GetConfig
	} else {
		manager.getConfig = func() ConfigSnapshot { return ConfigSnapshot{} }
	}
	if options.ListAuths != nil {
		manager.listAuths = options.ListAuths
	} else {
		manager.listAuths = func() []*coreauth.Auth { return nil }
	}
	if options.Now != nil {
		manager.now = options.Now
	} else {
		manager.now = time.Now
	}
	manager.backoff = defaultBackoffSchedule(options.Backoff)
	return manager
}

func (m *AutoManager) CurrentJob() *Job {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneJob(m.pending)
}

func (m *AutoManager) RunOnce(ctx context.Context) error {
	return m.run(ctx, false)
}

func (m *AutoManager) RunManual(ctx context.Context) error {
	return m.run(ctx, true)
}

func (m *AutoManager) run(ctx context.Context, force bool) error {
	if m == nil {
		return nil
	}
	if m.client == nil {
		return fmt.Errorf("job service client is nil")
	}

	cfg := m.getConfig()
	if !cfg.Enabled && !force {
		return nil
	}
	if cfg.TargetAccountCount <= 0 {
		return nil
	}
	if !force && !m.readyForAttempt() {
		return nil
	}

	pending := m.CurrentJob()
	if pending != nil {
		if err := m.reconcileCurrentPending(ctx); err != nil {
			m.recordFailure()
			return err
		}
		m.ensurePendingWatcher()
		m.resetFailure()
		return nil
	}

	snapshot := SummarizeAuthPool(m.listAuths())
	snapshot.Target = cfg.TargetAccountCount
	deficit := ComputeDeficit(snapshot)
	if deficit <= 0 {
		return nil
	}

	job, err := m.client.CreateJob(ctx, CreateJobInput{
		RequestedSuccesses: deficit,
		Source:             "auto",
		ZipRequired:        true,
	})
	if err != nil {
		m.recordFailure()
		return err
	}

	m.setPending(job)
	m.resetFailure()
	return nil
}

func (m *AutoManager) State() StateSnapshot {
	if m == nil {
		return StateSnapshot{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return StateSnapshot{
		CurrentJob:    cloneJob(m.pending),
		FailureCount:  m.failureCount,
		NextAttemptAt: m.nextAttemptAt,
	}
}

func (m *AutoManager) reconcilePending(ctx context.Context, pending *Job) error {
	job, err := m.client.GetJob(ctx, pending.ID)
	if err != nil {
		return err
	}
	if job == nil {
		return nil
	}

	m.setPending(job)

	switch strings.ToLower(strings.TrimSpace(job.Status)) {
	case "completed", "partial":
		archive, err := m.client.DownloadArchive(ctx, job.ID)
		if err != nil {
			return err
		}
		if m.importArchive != nil {
			if err = m.importArchive(ctx, archive); err != nil {
				return err
			}
		}
		m.clearPending()
	case "failed", "cancelled":
		m.clearPending()
	}
	return nil
}

func (m *AutoManager) reconcileCurrentPending(ctx context.Context) error {
	if m == nil {
		return nil
	}
	m.reconcileMu.Lock()
	defer m.reconcileMu.Unlock()

	pending := m.CurrentJob()
	if pending == nil {
		return nil
	}
	return m.reconcilePending(ctx, pending)
}

func (m *AutoManager) ensurePendingWatcher() {
	if m == nil {
		return
	}

	m.mu.Lock()
	if m.pending == nil || m.watchCancel != nil {
		m.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.watchCancel = cancel
	m.mu.Unlock()

	go m.watchPendingCompletion(ctx)
}

func (m *AutoManager) watchPendingCompletion(ctx context.Context) {
	interval := pendingReconcilePollInterval
	if interval <= 0 {
		interval = 2 * time.Second
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		pollTimeout := pendingReconcileRequestTimeout
		if pollTimeout <= 0 {
			pollTimeout = 30 * time.Second
		}
		pollCtx, cancel := context.WithTimeout(context.Background(), pollTimeout)
		_ = m.reconcileCurrentPending(pollCtx)
		cancel()
		if m.CurrentJob() == nil {
			return
		}
		timer.Reset(interval)
	}
}

func (m *AutoManager) setPending(job *Job) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.pending = cloneJob(job)
	m.mu.Unlock()
	m.ensurePendingWatcher()
}

func (m *AutoManager) clearPending() {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.pending = nil
	cancel := m.watchCancel
	m.watchCancel = nil
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func cloneJob(job *Job) *Job {
	if job == nil {
		return nil
	}
	copied := *job
	return &copied
}

func (m *AutoManager) readyForAttempt() bool {
	if m == nil {
		return false
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.nextAttemptAt.IsZero() || !now.Before(m.nextAttemptAt)
}

func (m *AutoManager) recordFailure() {
	if m == nil {
		return
	}
	now := m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureCount++
	delay := m.backoff[len(m.backoff)-1]
	if idx := m.failureCount - 1; idx >= 0 && idx < len(m.backoff) {
		delay = m.backoff[idx]
	}
	m.nextAttemptAt = now.Add(delay)
}

func (m *AutoManager) resetFailure() {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failureCount = 0
	m.nextAttemptAt = time.Time{}
}

func defaultBackoffSchedule(schedule []time.Duration) []time.Duration {
	if len(schedule) == 0 {
		return []time.Duration{
			time.Minute,
			3 * time.Minute,
			10 * time.Minute,
			30 * time.Minute,
		}
	}
	out := make([]time.Duration, 0, len(schedule))
	for _, delay := range schedule {
		if delay > 0 {
			out = append(out, delay)
		}
	}
	if len(out) == 0 {
		return []time.Duration{time.Minute}
	}
	return out
}

// Package cliproxy provides the Codex-only service implementation for the CLI Proxy API.
// It owns the proxy lifecycle, runtime auth state, file watching, and HTTP server.
package cliproxy

import (
	"context"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/watcher"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
)

// WatcherFactory creates a watcher for configuration and token changes.
// The reload callback receives the updated configuration when changes are detected.
//
// Parameters:
//   - configPath: The path to the configuration file to watch
//   - authDir: The directory containing authentication tokens to watch
//   - reload: The callback function to call when changes are detected
//
// Returns:
//   - *WatcherWrapper: A watcher wrapper instance
//   - error: An error if watcher creation fails
type WatcherFactory func(configPath, authDir string, reload func(*config.Config)) (*WatcherWrapper, error)

// WatcherWrapper exposes the subset of watcher methods required by the SDK.
type WatcherWrapper struct {
	start func(ctx context.Context) error
	stop  func() error

	setConfig             func(cfg *config.Config)
	snapshotAuths         func() []*coreauth.Auth
	setUpdateQueue        func(queue chan<- watcher.AuthUpdate)
	dispatchRuntimeUpdate func(update watcher.AuthUpdate) bool
}

// Start proxies to the underlying watcher Start implementation.
func (w *WatcherWrapper) Start(ctx context.Context) error {
	if w == nil || w.start == nil {
		return nil
	}
	return w.start(ctx)
}

// Stop proxies to the underlying watcher Stop implementation.
func (w *WatcherWrapper) Stop() error {
	if w == nil || w.stop == nil {
		return nil
	}
	return w.stop()
}

// SetConfig updates the watcher configuration cache.
func (w *WatcherWrapper) SetConfig(cfg *config.Config) {
	if w == nil || w.setConfig == nil {
		return
	}
	w.setConfig(cfg)
}

// DispatchRuntimeAuthUpdate forwards runtime auth updates (for example websocket refreshes)
// into the watcher-managed auth update queue when available.
// Returns true if the update was enqueued successfully.
func (w *WatcherWrapper) DispatchRuntimeAuthUpdate(update watcher.AuthUpdate) bool {
	if w == nil || w.dispatchRuntimeUpdate == nil {
		return false
	}
	return w.dispatchRuntimeUpdate(update)
}

// SnapshotAuths returns the current auth entries derived from watched files.
func (w *WatcherWrapper) SnapshotAuths() []*coreauth.Auth {
	if w == nil || w.snapshotAuths == nil {
		return nil
	}
	return w.snapshotAuths()
}

// SetAuthUpdateQueue registers the channel used to propagate auth updates.
func (w *WatcherWrapper) SetAuthUpdateQueue(queue chan<- watcher.AuthUpdate) {
	if w == nil || w.setUpdateQueue == nil {
		return
	}
	w.setUpdateQueue(queue)
}

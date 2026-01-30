// Package config provides configuration loading, parsing, and hot-reload for cc-relay.
package config

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
)

// ReloadCallback is called when the config file changes and is successfully reloaded.
// The callback receives the new configuration. If the callback returns an error,
// it will be logged but the config reload is still considered successful.
type ReloadCallback func(*Config) error

// ErrWatcherClosed is returned when an operation is attempted on a closed watcher.
var ErrWatcherClosed = errors.New("config: watcher already closed")

// Watcher monitors a config file for changes and triggers reload callbacks.
// It handles debouncing of rapid file changes (common with editors) and
// watches the parent directory to properly detect atomic writes.
type Watcher struct {
	ctx           context.Context
	fsWatcher     *fsnotify.Watcher
	cancel        context.CancelFunc
	path          string
	callbacks     []ReloadCallback
	debounceDelay time.Duration
	mu            sync.RWMutex
	closed        bool
}

// WatcherOption configures a Watcher.
type WatcherOption func(*Watcher)

// WithDebounceDelay sets the debounce delay for file change events.
// Default is 100ms. A longer delay helps with editors that trigger multiple events.
func WithDebounceDelay(d time.Duration) WatcherOption {
	return func(w *Watcher) {
		w.debounceDelay = d
	}
}

// NewWatcher creates a new config file watcher for the given path.
// The path is resolved to an absolute path. The watcher monitors the parent
// directory to properly detect atomic writes (temp file + rename pattern).
func NewWatcher(path string, opts ...WatcherOption) (*Watcher, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	w := &Watcher{
		path:          absPath,
		fsWatcher:     fsWatcher,
		callbacks:     make([]ReloadCallback, 0),
		debounceDelay: 100 * time.Millisecond,
		ctx:           ctx,
		cancel:        cancel,
	}

	for _, opt := range opts {
		opt(w)
	}

	// Watch parent directory to catch atomic writes (temp + rename pattern)
	dir := filepath.Dir(absPath)
	if err := fsWatcher.Add(dir); err != nil {
		if closeErr := fsWatcher.Close(); closeErr != nil {
			log.Error().Err(closeErr).Msg("failed to close watcher after add failure")
		}
		return nil, err
	}

	return w, nil
}

// Path returns the absolute path being watched.
func (w *Watcher) Path() string {
	return w.path
}

// OnReload registers a callback to be invoked when the config file is reloaded.
// Multiple callbacks can be registered and will be called in order.
// The callback is called with the new configuration after successful parsing.
func (w *Watcher) OnReload(cb ReloadCallback) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.callbacks = append(w.callbacks, cb)
}

// Watch starts watching for config file changes.
// It blocks until the context is canceled. Events are debounced to handle
// editors that trigger multiple events on save. Only Write and Create events
// are processed (Chmod is ignored). Returns nil when context is canceled.
func (w *Watcher) Watch(ctx context.Context) error {
	var (
		timer      *time.Timer
		pending    bool
		timerMu    sync.Mutex
		targetFile = filepath.Base(w.path)
	)

	for {
		select {
		case <-ctx.Done():
			w.cleanupTimer(timer)
			return nil

		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return nil
			}
			if w.shouldProcessEvent(event, targetFile) {
				w.handleEvent(&timerMu, &timer, &pending)
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return nil
			}
			log.Error().Err(err).Msg("config watcher error")
		}
	}
}

// shouldProcessEvent determines if an fsnotify event should trigger a reload.
// It checks if the event is for our target file and is a Write or Create event.
func (w *Watcher) shouldProcessEvent(event fsnotify.Event, targetFile string) bool {
	// Only process events for our config file
	if filepath.Base(event.Name) != targetFile {
		return false
	}

	// Only Write and Create events trigger reload (ignore Chmod from indexers/antivirus)
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Create)
}

// handleEvent processes a file change event with debouncing.
func (w *Watcher) handleEvent(timerMu *sync.Mutex, timer **time.Timer, pending *bool) {
	timerMu.Lock()
	defer timerMu.Unlock()

	// If timer exists, reset it (extend debounce window)
	if *timer != nil {
		(*timer).Stop()
	}

	*pending = true
	*timer = time.AfterFunc(w.debounceDelay, func() {
		// Check if watcher is still active before triggering reload.
		// This prevents goroutine leak when timer fires after watcher is closed.
		select {
		case <-w.ctx.Done():
			return // Watcher is closed, don't trigger reload
		default:
		}
		timerMu.Lock()
		*pending = false
		timerMu.Unlock()
		w.triggerReload()
	})
}

// cleanupTimer safely stops and cleans up the debounce timer.
func (w *Watcher) cleanupTimer(timer *time.Timer) {
	if timer != nil {
		timer.Stop()
	}
}

// triggerReload loads the config and invokes all registered callbacks.
func (w *Watcher) triggerReload() {
	cfg, err := Load(w.path)
	if err != nil {
		log.Error().Err(err).Str("path", w.path).Msg("failed to reload config")
		return
	}

	log.Info().Str("path", w.path).Msg("config file reloaded")
	w.invokeCallbacks(cfg)
}

// invokeCallbacks calls all registered callbacks with the new config.
func (w *Watcher) invokeCallbacks(cfg *Config) {
	w.mu.RLock()
	callbacks := make([]ReloadCallback, len(w.callbacks))
	copy(callbacks, w.callbacks)
	w.mu.RUnlock()

	for _, cb := range callbacks {
		if err := cb(cfg); err != nil {
			log.Error().Err(err).Msg("config reload callback error")
		}
	}
}

// Close stops watching and releases resources.
// Returns ErrWatcherClosed if already closed.
func (w *Watcher) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.closed {
		return ErrWatcherClosed
	}
	w.closed = true

	// Cancel context to prevent any pending timer callbacks from triggering reload
	w.cancel()

	return w.fsWatcher.Close()
}

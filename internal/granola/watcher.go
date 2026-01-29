package granola

import (
	"log/slog"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors the Granola cache file for changes
type Watcher struct {
	path           string
	debounce       time.Duration
	onChange       func()
	watcher        *fsnotify.Watcher
	stop           chan struct{}
	stopped        chan struct{}
	mu             sync.Mutex
	lastEventTime  time.Time
	pendingTrigger bool
}

// NewWatcher creates a new file watcher with debouncing
func NewWatcher(path string, debounceSeconds int, onChange func()) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		path:     path,
		debounce: time.Duration(debounceSeconds) * time.Second,
		onChange: onChange,
		watcher:  fsWatcher,
		stop:     make(chan struct{}),
		stopped:  make(chan struct{}),
	}

	return w, nil
}

// Start begins watching the file
func (w *Watcher) Start() error {
	if err := w.watcher.Add(w.path); err != nil {
		return err
	}

	go w.run()
	return nil
}

// Stop stops the watcher
func (w *Watcher) Stop() {
	close(w.stop)
	<-w.stopped
	w.watcher.Close()
}

func (w *Watcher) run() {
	defer close(w.stopped)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-w.stop:
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			// Trigger on WRITE events (file content changed)
			if event.Has(fsnotify.Write) {
				w.mu.Lock()
				w.lastEventTime = time.Now()
				w.pendingTrigger = true
				w.mu.Unlock()
				slog.Debug("cache file changed", "event", event.Op.String())
			}

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "error", err)

		case <-ticker.C:
			w.mu.Lock()
			if w.pendingTrigger && time.Since(w.lastEventTime) >= w.debounce {
				w.pendingTrigger = false
				w.mu.Unlock()
				slog.Info("triggering sync after debounce")
				w.onChange()
			} else {
				w.mu.Unlock()
			}
		}
	}
}

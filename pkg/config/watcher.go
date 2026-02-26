package config

import (
	"fmt"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileWatcher struct {
	watcher   *fsnotify.Watcher
	callbacks map[string][]func()
	mu        sync.RWMutex
	running   bool
	stopCh    chan struct{}
}

func NewFileWatcher() *FileWatcher {
	return &FileWatcher{
		callbacks: make(map[string][]func()),
		stopCh:    make(chan struct{}),
	}
}

func (w *FileWatcher) Start() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	w.watcher = watcher
	w.running = true

	return nil
}

func (w *FileWatcher) watchLoop() {
	var debounceTimer *time.Timer
	pendingPaths := make(map[string]bool)
	debounceMutex := sync.Mutex{}

	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.mu.RLock()
			_, exists := w.callbacks[event.Name]
			w.mu.RUnlock()

			if exists {
				debounceMutex.Lock()
				pendingPaths[event.Name] = true

				if debounceTimer == nil {
					debounceTimer = time.AfterFunc(100*time.Millisecond, func() {
						debounceMutex.Lock()
						paths := make([]string, 0, len(pendingPaths))

						for path := range pendingPaths {
							paths = append(paths, path)
						}
						pendingPaths = make(map[string]bool)
						debounceMutex.Unlock()

						w.mu.RLock()
						for _, path := range paths {
							for _, cp := range w.callbacks[path] {
								go cp()
							}
						}

						w.mu.RUnlock()

					})
					debounceMutex.Unlock()
				}

			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("file watcher error: %v\n", err)

		}
	}
}

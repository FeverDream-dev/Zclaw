package connections

import (
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileEventType represents the kind of filesystem event observed.
type FileEventType string

const (
	Create FileEventType = "create"
	Modify FileEventType = "modify"
	Delete FileEventType = "delete"
	Rename FileEventType = "rename"
)

// FileEvent describes a filesystem change.
type FileEvent struct {
	Type      FileEventType
	Path      string
	Timestamp time.Time
}

// FileWatcher watches directories for changes matching simple glob-like patterns.
type FileWatcher struct {
	mu       sync.RWMutex
	watchers map[string]*watchContext
	events   chan FileEvent
	interval time.Duration
}

type watchContext struct {
	path    string
	pattern string
	last    map[string]int64
	active  bool
}

// NewFileWatcher creates a new FileWatcher with a default polling interval.
func NewFileWatcher() *FileWatcher {
	return &FileWatcher{watchers: make(map[string]*watchContext), events: make(chan FileEvent, 128), interval: 2 * time.Second}
}

// Watch starts watching the given path for files that match the provided pattern.
func (fw *FileWatcher) Watch(path, pattern string) error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	key := path + "::" + pattern
	if _, ok := fw.watchers[key]; ok {
		return nil
	}
	wc := &watchContext{path: path, pattern: pattern, last: make(map[string]int64), active: true}
	fw.watchers[key] = wc
	go fw.loopWatch(key, wc)
	return nil
}

// loopWatch runs a polling loop for a single watcher.
func (fw *FileWatcher) loopWatch(key string, wc *watchContext) {
	ticker := time.NewTicker(fw.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			fw.scanWatcher(wc)
		}
		// exit if watcher marked inactive
		fw.mu.RLock()
		active := wc.active
		fw.mu.RUnlock()
		if !active {
			return
		}
	}
}

// scanWatcher scans the watcher for changes and emits events.
func (fw *FileWatcher) scanWatcher(wc *watchContext) {
	current := make(map[string]int64)
	_ = filepath.WalkDir(wc.path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		base := filepath.Base(p)
		matched, _ := filepath.Match(wc.pattern, base)
		if matched {
			info, err := os.Stat(p)
			if err == nil {
				current[p] = info.ModTime().UnixNano()
			}
		}
		return nil
	})
	// Detect creations and modifications
	for path, mtime := range current {
		if prev, ok := wc.last[path]; !ok {
			fw.publish(FileEvent{Type: Create, Path: path, Timestamp: time.Unix(0, mtime)})
		} else if mtime != prev {
			fw.publish(FileEvent{Type: Modify, Path: path, Timestamp: time.Unix(0, mtime)})
		}
	}
	// Detect deletions
	for path := range wc.last {
		if _, ok := current[path]; !ok {
			fw.publish(FileEvent{Type: Delete, Path: path, Timestamp: time.Now()})
		}
	}
	wc.last = current
}

// fwemit is a helper to publish an event to the common Events channel.
func (fw *FileWatcher) publish(e FileEvent) {
	select {
	case fw.events <- e:
	default:
	}
}

// Events returns a read-only channel of file events.
func (fw *FileWatcher) Events() <-chan FileEvent {
	return fw.events
}

// Stop stops all watchers and releases resources.
func (fw *FileWatcher) Stop() error {
	fw.mu.Lock()
	defer fw.mu.Unlock()
	for _, wc := range fw.watchers {
		wc.active = false
	}
	close(fw.events)
	return nil
}

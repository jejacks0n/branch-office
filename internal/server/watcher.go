package server

import (
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"branch-office/internal/git"
	"github.com/fsnotify/fsnotify"
)

type RepoWatcher struct {
	mu       sync.Mutex
	repoPath string
	watcher  *fsnotify.Watcher
	subs     map[chan struct{}]struct{}
	stopChan chan struct{}
}

type WatcherManager struct {
	mu       sync.Mutex
	watchers map[string]*RepoWatcher
}

func NewWatcherManager() *WatcherManager {
	return &WatcherManager{
		watchers: make(map[string]*RepoWatcher),
	}
}

func (m *WatcherManager) Subscribe(repoPath string) (chan struct{}, func(), error) {
	m.mu.Lock()
	cleanPath := filepath.Clean(repoPath)
	w, exists := m.watchers[cleanPath]
	if !exists {
		var err error
		w, err = newRepoWatcher(cleanPath)
		if err != nil {
			m.mu.Unlock()
			return nil, nil, err
		}
		m.watchers[cleanPath] = w
	}
	m.mu.Unlock()

	ch := make(chan struct{}, 5)
	w.mu.Lock()
	w.subs[ch] = struct{}{}
	w.mu.Unlock()

	unsubscribe := func() {
		w.mu.Lock()
		delete(w.subs, ch)
		close(ch)
		remaining := len(w.subs)
		w.mu.Unlock()

		if remaining == 0 {
			m.mu.Lock()
			// Double-check if still 0
			w.mu.Lock()
			if len(w.subs) == 0 {
				w.close()
				delete(m.watchers, cleanPath)
			}
			w.mu.Unlock()
			m.mu.Unlock()
		}
	}

	return ch, unsubscribe, nil
}

func newRepoWatcher(repoPath string) (*RepoWatcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	rw := &RepoWatcher{
		repoPath: repoPath,
		watcher:  fsWatcher,
		subs:     make(map[chan struct{}]struct{}),
		stopChan: make(chan struct{}),
	}

	if err := rw.addWatches(); err != nil {
		fsWatcher.Close()
		return nil, err
	}

	// Also watch HEAD and refs in git directory (handles linked worktrees where .git is a file pointing to .git/worktrees/<name>)
	client := git.NewClient(repoPath)
	if gitDir, err := client.GetGitDir(); err == nil && gitDir != "" {
		headFile := filepath.Join(gitDir, "HEAD")
		if _, err := os.Stat(headFile); err == nil {
			_ = fsWatcher.Add(headFile)
		}
		refsDir := filepath.Join(gitDir, "refs")
		if fi, err := os.Stat(refsDir); err == nil && fi.IsDir() {
			_ = fsWatcher.Add(refsDir)
			headsDir := filepath.Join(refsDir, "heads")
			if fi, err := os.Stat(headsDir); err == nil && fi.IsDir() {
				_ = fsWatcher.Add(headsDir)
			}
		}
	}

	go rw.loop()

	return rw, nil
}

func (rw *RepoWatcher) addWatches() error {
	return filepath.WalkDir(rw.repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		name := d.Name()
		if path != rw.repoPath {
			// Skip noisy or ignored directories
			if name == ".git" {
				// Do not watch .git directory with kqueue to prevent status lock feedback loops
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "dist", "bin", "vendor", "build", "target", ".wrangler", ".next", ".nuxt", ".cache", ".turbo":
				return filepath.SkipDir
			}
		}

		_ = rw.watcher.Add(path)
		return nil
	})
}

func (rw *RepoWatcher) loop() {
	var debounceTimer *time.Timer
	var timerMu sync.Mutex

	trigger := func() {
		timerMu.Lock()
		defer timerMu.Unlock()

		if debounceTimer != nil {
			debounceTimer.Stop()
		}
		debounceTimer = time.AfterFunc(200*time.Millisecond, func() {
			rw.broadcast()
		})
	}

	for {
		select {
		case <-rw.stopChan:
			timerMu.Lock()
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			timerMu.Unlock()
			return

		case event, ok := <-rw.watcher.Events:
			if !ok {
				return
			}

			if shouldIgnoreEvent(rw.repoPath, event.Name) {
				continue
			}

			// If a new directory was created, watch it
			if event.Has(fsnotify.Create) {
				if fi, err := os.Stat(event.Name); err == nil && fi.IsDir() {
					if !shouldIgnoreEvent(rw.repoPath, event.Name) {
						_ = rw.watcher.Add(event.Name)
					}
				}
			}

			log.Printf("[WATCHER] Change detected: %s (op: %s)", event.Name, event.Op)
			trigger()

		case _, ok := <-rw.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

func shouldIgnoreEvent(repoPath, eventPath string) bool {
	cleanEvent := filepath.Clean(eventPath)
	cleanRepo := filepath.Clean(repoPath)

	rel, err := filepath.Rel(cleanRepo, cleanEvent)
	if err != nil {
		return false
	}

	parts := strings.Split(rel, string(filepath.Separator))
	for i, part := range parts {
		if part == "" || part == "." {
			continue
		}

		// Inside git directory: only trigger on true ref/HEAD updates, never on .git directory itself or index
		if part == ".git" {
			if i == len(parts)-1 {
				return true
			}
			sub := parts[i+1]
			if sub == "HEAD" || sub == "refs" {
				return false
			}
			return true
		}

		// Ignore hidden directories or files (e.g. .wrangler, .vscode, .idea)
		if strings.HasPrefix(part, ".") {
			return true
		}

		// Ignore common high-churn build/dependency folders
		switch part {
		case "node_modules", "dist", "bin", "vendor", "build", "target", ".wrangler", ".next", ".nuxt", ".cache", ".turbo":
			return true
		}
	}

	base := filepath.Base(cleanEvent)
	if strings.HasSuffix(base, ".tmp") || strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, "~") || strings.HasPrefix(base, ".#") || strings.HasSuffix(base, ".lock") || base == "index" {
		return true
	}

	return false
}

func (rw *RepoWatcher) broadcast() {
	rw.mu.Lock()
	defer rw.mu.Unlock()

	for ch := range rw.subs {
		select {
		case ch <- struct{}{}:
		default:
			// Non-blocking drop if channel buffer full
		}
	}
}

func (rw *RepoWatcher) close() {
	close(rw.stopChan)
	_ = rw.watcher.Close()
}

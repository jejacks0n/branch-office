package config

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type Repo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type Config struct {
	SSHKey         string `json:"sshKey,omitempty"`
	DisableSigning bool   `json:"disableSigning,omitempty"`
	Repositories   []Repo `json:"repositories"`
}

type Store struct {
	mu         sync.RWMutex
	configPath string
	data       Config
}

func GetDefaultConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "broffice", "config.json"), nil
}

func NewStore(customPath string) (*Store, error) {
	path := customPath
	if path == "" {
		if envPath := os.Getenv("BROFFICE_CONFIG"); envPath != "" {
			path = envPath
		} else if defaultPath, err := GetDefaultConfigPath(); err == nil {
			path = defaultPath
		}
	}

	store := &Store{
		configPath: path,
		data:       Config{Repositories: []Repo{}},
	}

	if err := store.load(); err != nil {
		if customPath == "" {
			// Gracefully fallback to local .broffice/config.json in current working directory
			cwd, _ := os.Getwd()
			fallbackPath := filepath.Join(cwd, ".broffice", "config.json")
			store.configPath = fallbackPath
			if errFallback := store.load(); errFallback == nil {
				return store, nil
			}
		}
		return nil, err
	}

	return store, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := os.Stat(s.configPath); os.IsNotExist(err) {
		dir := filepath.Dir(s.configPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		return s.saveLocked()
	}

	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if cfg.Repositories == nil {
		cfg.Repositories = []Repo{}
	}
	s.data = cfg
	return nil
}

func (s *Store) saveLocked() error {
	data, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	tmpFile := s.configPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config temp file: %w", err)
	}

	if err := os.Rename(tmpFile, s.configPath); err != nil {
		return fmt.Errorf("failed to replace config file: %w", err)
	}

	return nil
}

func (s *Store) ListRepos() []Repo {
	s.mu.RLock()
	defer s.mu.RUnlock()

	res := make([]Repo, len(s.data.Repositories))
	copy(res, s.data.Repositories)
	return res
}

func (s *Store) GetRepo(id string) (*Repo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, r := range s.data.Repositories {
		if r.ID == id {
			copyRepo := r
			return &copyRepo, nil
		}
	}
	return nil, errors.New("repository not found")
}

func (s *Store) GetSSHKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.SSHKey
}

func (s *Store) SetSSHKey(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.SSHKey = key
	return s.saveLocked()
}

func (s *Store) GetDisableSigning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data.DisableSigning
}

func (s *Store) SetDisableSigning(disable bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data.DisableSigning = disable
	return s.saveLocked()
}

func ValidateAndResolveGitPath(targetPath string) (string, error) {
	cleanPath := filepath.Clean(targetPath)
	if strings.HasPrefix(cleanPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		cleanPath = filepath.Join(home, strings.TrimPrefix(cleanPath, "~"))
	}

	absPath, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	fi, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("directory does not exist: %w", err)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", absPath)
	}

	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = absPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("not a git repository: %s", absPath)
	}

	topLevel := strings.TrimSpace(string(out))
	return filepath.Clean(topLevel), nil
}

func HashPath(path string) string {
	sum := sha256.Sum256([]byte(path))
	return hex.EncodeToString(sum[:8])
}

func (s *Store) AddRepo(inputPath string) (*Repo, error) {
	resolvedPath, err := ValidateAndResolveGitPath(inputPath)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	repoID := HashPath(resolvedPath)
	name := filepath.Base(resolvedPath)

	for _, existing := range s.data.Repositories {
		if existing.ID == repoID || existing.Path == resolvedPath {
			copyRepo := existing
			return &copyRepo, nil
		}
	}

	repo := Repo{
		ID:   repoID,
		Name: name,
		Path: resolvedPath,
	}

	s.data.Repositories = append(s.data.Repositories, repo)
	if err := s.saveLocked(); err != nil {
		return nil, err
	}

	return &repo, nil
}

func (s *Store) RemoveRepo(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := -1
	for i, r := range s.data.Repositories {
		if r.ID == id {
			index = i
			break
		}
	}

	if index == -1 {
		return errors.New("repository not found")
	}

	s.data.Repositories = append(s.data.Repositories[:index], s.data.Repositories[index+1:]...)
	return s.saveLocked()
}

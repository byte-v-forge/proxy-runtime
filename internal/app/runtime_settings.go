package app

import (
	"context"
	"log/slog"
	"sync"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type runtimeSettingsFile = proxyruntimev1.ProxyRuntimePersistentSettings

const defaultProxyExitIPTimeout = 5 * time.Second

type runtimeSettingsStore struct {
	store  *PostgresStore
	logger *slog.Logger
	mu     sync.Mutex
}

func newRuntimeSettingsStore(store *PostgresStore, logger *slog.Logger) *runtimeSettingsStore {
	if logger == nil {
		logger = slog.Default()
	}
	return &runtimeSettingsStore{store: store, logger: logger}
}

func (s *runtimeSettingsStore) view() (*proxyruntimev1.ProxyRuntimeSettings, error) {
	settings, err := s.load()
	if err != nil {
		return nil, err
	}
	return runtimeSettingsView(settings), nil
}

func (s *runtimeSettingsStore) update(req *proxyruntimev1.UpdateProxyRuntimeSettingsRequest) (*proxyruntimev1.ProxyRuntimeSettings, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	current, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	settings, err := settingsFromRequest(req, current)
	if err != nil {
		return nil, err
	}
	if err := s.saveLocked(settings); err != nil {
		return nil, err
	}
	return runtimeSettingsView(settings), nil
}

func (s *runtimeSettingsStore) load() (*runtimeSettingsFile, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *runtimeSettingsStore) loadLocked() (*runtimeSettingsFile, error) {
	if s.store == nil {
		return normalizeRuntimeSettings(nil), nil
	}
	return s.store.LoadRuntimeSettings(context.Background())
}

func (s *runtimeSettingsStore) save(settings *runtimeSettingsFile) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked(settings)
}

func (s *runtimeSettingsStore) saveLocked(settings *runtimeSettingsFile) error {
	if s.store == nil {
		return nil
	}
	return s.store.SaveRuntimeSettings(context.Background(), normalizeRuntimeSettings(settings))
}

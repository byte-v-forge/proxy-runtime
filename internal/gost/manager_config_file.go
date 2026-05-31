package gost

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func marshalConfig(cfg Config) ([]byte, error) {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal gost config: %w", err)
	}
	return data, nil
}

func (m *Manager) writeConfigBytes(data []byte) (string, error) {
	dir := m.cfg.ConfigDir
	if dir == "" {
		dir = filepath.Join(os.TempDir(), "proxy-runtime")
	}
	path := filepath.Join(dir, "gost.json")
	return path, writeConfigData(path, data)
}

func writeConfigData(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create gost config dir: %w", err)
	}
	file, err := os.CreateTemp(dir, ".gost-*.json")
	if err != nil {
		return fmt.Errorf("create gost config file: %w", err)
	}
	tempPath := file.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return fmt.Errorf("write gost config file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close gost config file: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace gost config file: %w", err)
	}
	return nil
}

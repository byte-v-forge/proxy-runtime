package gost

import (
	"bytes"
	"context"
	"time"
)

func (m *Manager) Reload(ctx context.Context, cfg Config) error {
	data, err := marshalConfig(cfg)
	if err != nil {
		return err
	}

	m.mu.Lock()
	current := m.current
	if current != nil && current.exited {
		m.current = nil
		current = nil
	}
	m.mu.Unlock()

	if current != nil {
		if bytes.Equal(current.configData, data) {
			return nil
		}
		if err := writeConfigData(current.configPath, data); err != nil {
			return err
		}
		if err := reloadProcess(current); err != nil {
			return err
		}
		m.mu.Lock()
		if m.current == current {
			current.configData = append(current.configData[:0], data...)
		}
		m.mu.Unlock()
		return waitForServices(ctx, cfg.Services, 3*time.Second)
	}

	configPath, err := m.writeConfigBytes(data)
	if err != nil {
		return err
	}

	proc, err := m.start(ctx, configPath)
	if err != nil {
		return err
	}
	proc.configData = append([]byte(nil), data...)
	m.mu.Lock()
	m.current = proc
	m.mu.Unlock()
	go m.observe(proc)
	if err := waitForServices(ctx, cfg.Services, 3*time.Second); err != nil {
		m.mu.Lock()
		if m.current == proc {
			m.current = nil
		}
		m.mu.Unlock()
		_ = stopProcess(proc, 5*time.Second)
		return err
	}
	return nil
}

func (m *Manager) Stop() {
	m.mu.Lock()
	current := m.current
	m.current = nil
	m.mu.Unlock()
	if current != nil {
		_ = stopProcess(current, 5*time.Second)
	}
}

func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.current == nil {
		return Status{}
	}
	status := Status{Running: !m.current.exited, ConfigPath: m.current.configPath}
	if m.current.exitErr != nil {
		status.LastError = m.current.exitErr.Error()
	}
	return status
}

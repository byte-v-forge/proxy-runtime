package gost

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func (m *Manager) start(ctx context.Context, configPath string) (*process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	args := []string{"-C", configPath}
	if m.cfg.APIAddr != "" {
		args = append(args, "-api", m.cfg.APIAddr)
	}
	if m.cfg.MetricsAddr != "" {
		args = append(args, "-metrics", m.cfg.MetricsAddr)
	}
	cmd := exec.Command(m.cfg.GostPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start gost: %w", err)
	}
	return &process{cmd: cmd, configPath: configPath, done: make(chan error, 1)}, nil
}

func (m *Manager) observe(proc *process) {
	err := proc.cmd.Wait()
	proc.done <- err
	close(proc.done)

	m.mu.Lock()
	proc.exited = true
	proc.exitErr = err
	isCurrent := m.current == proc
	m.mu.Unlock()

	if isCurrent && err != nil && !errors.Is(err, context.Canceled) {
		m.logger.Warn("gost process exited", "error", err)
	}
}

func stopProcess(proc *process, timeout time.Duration) error {
	if proc.exited || proc.cmd.Process == nil {
		return nil
	}
	_ = proc.cmd.Process.Signal(syscall.SIGTERM)
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-proc.done:
		return nil
	case <-timer.C:
		_ = proc.cmd.Process.Kill()
		<-proc.done
		return nil
	}
}

func reloadProcess(proc *process) error {
	if proc.exited || proc.cmd.Process == nil {
		return errors.New("gost process is not running")
	}
	if err := proc.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		return fmt.Errorf("reload gost: %w", err)
	}
	return nil
}

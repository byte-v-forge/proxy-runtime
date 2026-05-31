package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func (d *Driver) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

func (d *Driver) Status() sourceplane.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return sourceplane.Status{Running: d.running, ConfigPath: d.configPath, LastError: d.lastError}
}

func (d *Driver) ensureConfigDir() (string, error) {
	if d.configDir != "" {
		return d.configDir, nil
	}
	dir := strings.TrimSpace(d.cfg.ConfigDir)
	if dir == "" {
		created, err := os.MkdirTemp("", "proxy-runtime-mihomo-")
		if err != nil {
			return "", err
		}
		dir = created
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	d.configDir = dir
	return dir, nil
}

func (d *Driver) startLocked(ctx context.Context, dir string, configPath string) error {
	path := strings.TrimSpace(d.cfg.Path)
	if path == "" {
		return errors.New("mihomo path is required")
	}
	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, path, "-f", configPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SAFE_PATHS="+dir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	d.cmd = cmd
	d.cancel = cancel
	d.running = true
	go d.wait(cmd)
	return nil
}

func (d *Driver) reloadLocked(ctx context.Context, configPath string) error {
	if strings.TrimSpace(d.cfg.APIAddr) == "" {
		return errors.New("mihomo api address is required for hot reload")
	}
	configPath = strings.TrimSpace(configPath)
	if !filepath.IsAbs(configPath) {
		return fmt.Errorf("mihomo reload config path must be absolute: %s", configPath)
	}
	body, err := json.Marshal(map[string]string{"path": configPath})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, controlURL(d.cfg.APIAddr, "/configs?force=true"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("mihomo config reload returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
}

func (d *Driver) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cmd != cmd {
		return
	}
	d.running = false
	if err != nil {
		d.lastError = err.Error()
	}
}

func (d *Driver) stopLocked() {
	cancel := d.cancel
	cmd := d.cmd
	d.cancel = nil
	d.cmd = nil
	d.running = false
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

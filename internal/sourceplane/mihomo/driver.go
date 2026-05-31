package mihomo

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

const (
	ProviderID            = "mihomo"
	groupName             = "byte-v-forge-source"
	nodeListenerPortStart = 10
	nodeListenerPortSpan  = 70
)

type Config struct {
	Path      string
	ConfigDir string
	APIAddr   string
}

type Driver struct {
	cfg    Config
	logger *slog.Logger

	mu           sync.Mutex
	cmd          *exec.Cmd
	cancel       context.CancelFunc
	configDir    string
	configPath   string
	signature    string
	sourceSig    string
	running      bool
	lastError    string
	lastEndpoint sourceplane.Endpoint
}

func New(cfg Config, logger *slog.Logger) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Driver{cfg: cfg, logger: logger}
}

func (d *Driver) Name() string { return ProviderID }

func (d *Driver) Reconcile(ctx context.Context, cfg sourceplane.Config) ([]provider.Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	file, err := d.loadSourceFileLocked(cfg.Providers)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	providers := enabledProviders(file.Subscriptions)
	fixedProxies := enabledFixedProxies(file.FixedProxies)
	if len(providers) == 0 && len(fixedProxies) == 0 {
		d.stopLocked()
		d.signature = ""
		d.sourceSig = ""
		d.lastError = "no mihomo sources configured"
		return nil, nil
	}
	endpoint, err := normalizeEndpoint(cfg.Endpoint)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	dir, err := d.ensureConfigDir()
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	baseOptions := renderOptions{Providers: providers, FixedProxies: fixedProxies, Endpoint: endpoint, ConfigDir: dir, APIAddr: d.cfg.APIAddr, GroupStrategy: cfg.GroupStrategy, HealthCheckURL: cfg.HealthCheckURL, HealthCheckInterval: cfg.HealthCheckInterval, HealthCheckTimeout: cfg.HealthCheckTimeout}
	configFile, err := renderConfig(baseOptions)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	data, err := json.MarshalIndent(configFile, "", "  ")
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	baseSig := signature(data)
	if err := os.MkdirAll(filepath.Join(dir, "providers"), 0o700); err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	configPath := filepath.Join(dir, "config.json")

	restartRequired := !d.running || d.lastEndpoint != endpoint
	sourceChanged := d.sourceSig != baseSig
	baseReloaded := false
	if restartRequired {
		if err := os.WriteFile(configPath, data, 0o600); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		d.stopLocked()
		if err := d.startLocked(ctx, dir, configPath); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		baseReloaded = true
	} else if sourceChanged {
		if err := os.WriteFile(configPath, data, 0o600); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		if err := d.reloadLocked(ctx, configPath); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		baseReloaded = true
	}
	if err := waitForEndpoint(ctx, endpoint.Addr, 3*time.Second); err != nil {
		d.lastError = err.Error()
		return nil, err
	}

	bindings, err := d.nodeListenersLocked(ctx, endpoint, providers, fixedProxies)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	finalOptions := baseOptions
	finalOptions.NodeListeners = bindings
	finalConfig, err := renderConfig(finalOptions)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	finalData, err := json.MarshalIndent(finalConfig, "", "  ")
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	finalSig := signature(finalData)
	if baseReloaded || finalSig != d.signature {
		if err := os.WriteFile(configPath, finalData, 0o600); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		if err := d.reloadLocked(ctx, configPath); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
		if err := waitForEndpoint(ctx, endpoint.Addr, 3*time.Second); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
	}
	d.signature = finalSig
	d.sourceSig = baseSig
	d.configPath = configPath
	d.lastEndpoint = endpoint
	d.lastError = ""
	return sourceNodes(bindings), nil
}

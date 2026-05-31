package gost

import (
	"log/slog"
	"net/http"
	"os/exec"
	"sync"
	"time"
)

type ManagerConfig struct {
	GostPath    string
	ConfigDir   string
	APIAddr     string
	MetricsAddr string
}

type Manager struct {
	cfg       ManagerConfig
	logger    *slog.Logger
	apiClient *http.Client

	mu      sync.Mutex
	current *process
}

type process struct {
	cmd        *exec.Cmd
	configPath string
	configData []byte
	done       chan error
	exited     bool
	exitErr    error
}

type Status struct {
	Running    bool
	ConfigPath string
	LastError  string
}

func NewManager(cfg ManagerConfig, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{cfg: cfg, logger: logger, apiClient: &http.Client{Timeout: 5 * time.Second}}
}

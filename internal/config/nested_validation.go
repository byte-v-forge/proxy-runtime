package config

import (
	"errors"
	"fmt"
	"strings"
)

func (c IPFraudConfig) validate() error {
	if c.Timeout <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_TIMEOUT_SECONDS must be > 0")
	}
	if c.CacheTTL <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_CACHE_TTL_SECONDS must be > 0")
	}
	if c.KeyCooldown <= 0 {
		return errors.New("PROXY_RUNTIME_IP_FRAUD_KEY_COOLDOWN_SECONDS must be > 0")
	}
	return nil
}

func (c SessionListenerConfig) validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return errors.New("PROXY_RUNTIME_SESSION_LISTEN_HOST is required")
	}
	if c.PortStart <= 0 || c.PortStart > 65535 {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_START must be between 1 and 65535")
	}
	if c.PortEnd <= 0 || c.PortEnd > 65535 {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_END must be between 1 and 65535")
	}
	if c.PortEnd < c.PortStart {
		return errors.New("PROXY_RUNTIME_SESSION_PORT_END must be >= PROXY_RUNTIME_SESSION_PORT_START")
	}
	return nil
}

func (c MihomoConfig) validate() error {
	if strings.TrimSpace(c.Path) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_PATH is required")
	}
	if strings.TrimSpace(c.MixedAddr) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_MIXED_ADDR is required")
	}
	if strings.TrimSpace(c.APIAddr) == "" {
		return errors.New("PROXY_RUNTIME_MIHOMO_API_ADDR is required")
	}
	switch strings.TrimSpace(c.GroupStrategy) {
	case "", "select", "url-test", "fallback", "load-balance":
	default:
		return fmt.Errorf("unsupported mihomo group strategy %q", c.GroupStrategy)
	}
	if c.HealthCheckInterval < 0 {
		return errors.New("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_INTERVAL_SECONDS must be >= 0")
	}
	if c.HealthCheckTimeout < 0 {
		return errors.New("PROXY_RUNTIME_MIHOMO_HEALTH_CHECK_TIMEOUT_SECONDS must be >= 0")
	}
	return nil
}

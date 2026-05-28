package accountproxy

import (
	"errors"
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func validateConfig(cfg Config, definition Definition) error {
	if strings.TrimSpace(cfg.Username) == "" || strings.TrimSpace(cfg.Password) == "" {
		return errors.New("username and password are required")
	}
	return nil
}

func (c Config) Validate() error {
	plugin, ok := Get(c.ProviderID)
	if !ok {
		return fmt.Errorf("unsupported provider_id %q", c.ProviderID)
	}
	return plugin.Validate(c)
}

func validateProtocol(protocol string) error {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "", "http", "socks5":
		return nil
	default:
		return fmt.Errorf("unsupported proxy protocol %q", protocol)
	}
}

func defaultProtocol(protocol string, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "http":
		return "http"
	case "socks5":
		return "socks5"
	}
	if strings.EqualFold(strings.TrimSpace(fallback), "http") {
		return "http"
	}
	return "socks5"
}

func protocolEnumWithDefault(protocol string, fallback string) proxyruntimev1.ProxyProtocol {
	if defaultProtocol(protocol, fallback) == "http" {
		return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_HTTP
	}
	return proxyruntimev1.ProxyProtocol_PROXY_PROTOCOL_SOCKS5
}

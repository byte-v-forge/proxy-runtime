package mihomo

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func renderFixedProxy(item sourceplane.FixedProxy) (map[string]any, error) {
	parsed, err := url.Parse(strings.TrimSpace(item.URI))
	if err != nil {
		return nil, fmt.Errorf("parse fixed proxy uri: %w", err)
	}
	if strings.ToLower(parsed.Scheme) != "vless" {
		return nil, fmt.Errorf("unsupported fixed proxy scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	portValue := parsed.Port()
	if host == "" || portValue == "" || parsed.User == nil {
		return nil, errors.New("vless uri requires uuid, host and port")
	}
	var port int
	if _, err := fmt.Sscanf(portValue, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid vless port %q", portValue)
	}
	name := safeID(item.ID)
	if name == "" {
		return nil, errors.New("fixed proxy id is required")
	}
	query := parsed.Query()
	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	network := strings.ToLower(firstNonEmpty(query.Get("type"), query.Get("network"), "tcp"))
	config := map[string]any{
		"name":       name,
		"type":       "vless",
		"server":     host,
		"port":       port,
		"uuid":       parsed.User.Username(),
		"udp":        true,
		"network":    network,
		"encryption": firstNonEmpty(query.Get("encryption"), "none"),
	}
	if flow := strings.TrimSpace(query.Get("flow")); flow != "" {
		config["flow"] = flow
	}
	if fingerprint := firstNonEmpty(query.Get("fp"), query.Get("client-fingerprint")); fingerprint != "" {
		config["client-fingerprint"] = fingerprint
	}
	if security == "tls" || security == "reality" {
		config["tls"] = true
	}
	if serverName := firstNonEmpty(query.Get("sni"), query.Get("servername")); serverName != "" {
		config["servername"] = serverName
	}
	if security == "reality" {
		reality := map[string]any{}
		if value := firstNonEmpty(query.Get("pbk"), query.Get("public-key")); value != "" {
			reality["public-key"] = value
		}
		if value := firstNonEmpty(query.Get("sid"), query.Get("short-id")); value != "" {
			reality["short-id"] = value
		}
		if len(reality) > 0 {
			config["reality-opts"] = reality
		}
	}
	switch network {
	case "ws", "websocket":
		config["network"] = "ws"
		opts := map[string]any{}
		if value := strings.TrimSpace(query.Get("path")); value != "" {
			opts["path"] = value
		}
		if value := strings.TrimSpace(query.Get("host")); value != "" {
			opts["headers"] = map[string]string{"Host": value}
		}
		if len(opts) > 0 {
			config["ws-opts"] = opts
		}
	case "grpc":
		opts := map[string]any{}
		if value := firstNonEmpty(query.Get("serviceName"), query.Get("service-name")); value != "" {
			opts["grpc-service-name"] = value
		}
		if len(opts) > 0 {
			config["grpc-opts"] = opts
		}
	case "tcp":
	default:
		return nil, fmt.Errorf("unsupported vless network %q", network)
	}
	return config, nil
}

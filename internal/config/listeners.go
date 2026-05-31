package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func validateListeners(listeners []EgressListener) error {
	seen := map[string]struct{}{}
	for index, listener := range listeners {
		id := strings.TrimSpace(listener.ID)
		if id == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].id is required", index)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("duplicate proxy runtime listener id %q", id)
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(listener.Addr) == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].addr is required", index)
		}
		protocol := normalizeConfigToken(listener.Protocol)
		if protocol == "" {
			protocol = "http"
		}
		if !isLocalProtocol(protocol) {
			return fmt.Errorf("unsupported listener protocol %q", listener.Protocol)
		}
		route := normalizeConfigToken(listener.Route)
		switch route {
		case "", ListenerRouteProvider, ListenerRouteDirect, ListenerRouteUpstream:
		default:
			return fmt.Errorf("unsupported listener route %q", listener.Route)
		}
		if route == ListenerRouteUpstream && strings.TrimSpace(listener.Upstream) == "" {
			return fmt.Errorf("PROXY_RUNTIME_LISTENERS_JSON[%d].upstream is required for upstream route", index)
		}
	}
	return nil
}

func envListeners(name string) []EgressListener {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	var listeners []EgressListener
	if err := json.Unmarshal([]byte(raw), &listeners); err != nil {
		return []EgressListener{{
			ID:    "__invalid__",
			Addr:  "invalid",
			Route: fmt.Sprintf("invalid JSON: %v", err),
		}}
	}
	for index := range listeners {
		listeners[index].ID = strings.TrimSpace(listeners[index].ID)
		listeners[index].Addr = strings.TrimSpace(listeners[index].Addr)
		listeners[index].Protocol = normalizeConfigToken(listeners[index].Protocol)
		listeners[index].Route = normalizeConfigToken(listeners[index].Route)
		listeners[index].Upstream = strings.TrimSpace(listeners[index].Upstream)
		listeners[index].Username = strings.TrimSpace(listeners[index].Username)
		listeners[index].Password = strings.TrimSpace(listeners[index].Password)
	}
	return listeners
}

func isLocalProtocol(protocol string) bool {
	switch protocol {
	case "http", "socks5":
		return true
	default:
		return false
	}
}

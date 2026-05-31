package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

func (r *Runtime) checkIPListener(listenerID string) (config.EgressListener, error) {
	configs := r.baseListenerConfigs()
	leases, _ := r.leases.ListLeases(context.Background(), false)
	for _, lease := range leases {
		if lease.GetListener() != nil {
			configs = append(configs, listenerFromProto(lease.GetListener()))
		}
	}
	if listenerID != "" {
		for _, listener := range configs {
			if listener.ID == listenerID {
				return listener, nil
			}
		}
		return config.EgressListener{}, fmt.Errorf("listener %q is not configured", listenerID)
	}
	for _, listener := range configs {
		if listenerRoute(listener) == config.ListenerRouteProvider {
			return listener, nil
		}
	}
	if len(configs) == 0 {
		return config.EgressListener{}, errors.New("no egress listener is configured")
	}
	return configs[0], nil
}

func (r *Runtime) localListenerEndpoint(listener config.EgressListener, advertisedHost string) (*proxyruntimev1.ProxyEndpoint, error) {
	hostPort, err := localListenHostPort(listener.Addr)
	if err != nil {
		return nil, err
	}
	host, portValue, err := net.SplitHostPort(hostPort)
	if err != nil {
		return nil, err
	}
	port, err := parsePort(portValue)
	if err != nil {
		return nil, err
	}
	if advertisedHost != "" && (host == "127.0.0.1" || host == "localhost" || host == "0.0.0.0" || host == "::1") {
		host = advertisedHost
	}
	return &proxyruntimev1.ProxyEndpoint{Id: listener.ID, Protocol: protocolFromName(listenerProtocol(listener, r.cfg.LocalProtocol)), Host: host, Port: port, Labels: cloneLabels(listener.Labels)}, nil
}

func (r *Runtime) sessionAdvertisedHost(req *http.Request, listener config.EgressListener) string {
	if host := strings.TrimSpace(r.cfg.SessionListener.AdvertisedHost); host != "" {
		return host
	}
	bindHost := listenerBindHost(listener.Addr)
	if bindHost != "" && !localOnlyHost(bindHost) {
		return bindHost
	}
	if unspecifiedBindHost(bindHost) {
		if host := firstLocalAdvertisedIP(); host != "" {
			return host
		}
	}
	return advertisedProxyHost(req)
}

func listenerBindHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return ""
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return ""
	}
	return strings.Trim(host, "[]")
}

func localOnlyHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	if host == "" || host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip == nil || ip.IsUnspecified() || ip.IsLoopback()
}

func unspecifiedBindHost(host string) bool {
	host = strings.Trim(strings.TrimSpace(strings.ToLower(host)), "[]")
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

func firstLocalAdvertisedIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	ipv6 := ""
	for _, addr := range addrs {
		ip := interfaceIP(addr)
		if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() || !ip.IsGlobalUnicast() {
			continue
		}
		if ipv4 := ip.To4(); ipv4 != nil {
			return ipv4.String()
		}
		if ipv6 == "" {
			ipv6 = ip.String()
		}
	}
	return ipv6
}

func interfaceIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}

func advertisedProxyHost(req *http.Request) string {
	if req == nil {
		return ""
	}
	host := strings.TrimSpace(req.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = strings.TrimSpace(req.Host)
	}
	if parsed, _, err := net.SplitHostPort(host); err == nil {
		return parsed
	}
	return host
}

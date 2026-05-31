package app

import (
	"context"
	"fmt"
	"net"

	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

func (r *Runtime) leaseListener(ctx context.Context, accountID string) (config.EgressListener, error) {
	leases, _ := r.leases.ListLeases(ctx, false)
	for _, lease := range leases {
		if lease.GetAccountId() == accountID && lease.GetListener() != nil {
			return listenerFromProto(lease.GetListener()), nil
		}
	}
	used := map[int]struct{}{}
	for _, lease := range leases {
		if _, portValue, err := net.SplitHostPort(lease.GetListener().GetListenAddr()); err == nil {
			var port int
			_, _ = fmt.Sscanf(portValue, "%d", &port)
			if port > 0 {
				used[port] = struct{}{}
			}
		}
	}
	span := r.cfg.SessionListener.PortEnd - r.cfg.SessionListener.PortStart + 1
	start := r.cfg.SessionListener.PortStart + int(hashModulo(accountID, uint32(span)))
	port := start
	for {
		if _, ok := used[port]; !ok && tcpPortAvailable(r.cfg.SessionListener.Host, port) {
			break
		}
		port++
		if port > r.cfg.SessionListener.PortEnd {
			port = r.cfg.SessionListener.PortStart
		}
		if port == start {
			return config.EgressListener{}, fmt.Errorf("no available proxy runtime session listener port in %d-%d", r.cfg.SessionListener.PortStart, r.cfg.SessionListener.PortEnd)
		}
	}
	id := "lease-" + shortHash(accountID)
	return config.EgressListener{ID: id, Addr: net.JoinHostPort(r.cfg.SessionListener.Host, fmt.Sprintf("%d", port)), Protocol: r.cfg.LocalProtocol, Route: config.ListenerRouteProvider, Labels: map[string]string{"mode": "dynamic_ip_session_lease", "account_id": accountID, "chain_id": leaseChainID(accountID)}}, nil
}

func tcpPortAvailable(host string, port int) bool {
	if port <= 0 {
		return false
	}
	addr := net.JoinHostPort(host, fmt.Sprintf("%d", port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return false
	}
	_ = listener.Close()
	return true
}

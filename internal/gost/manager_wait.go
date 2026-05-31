package gost

import (
	"context"
	"fmt"
	"net"
	"time"
)

func waitForServices(ctx context.Context, services []Service, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	targets := serviceListenTargets(services)
	if len(targets) == 0 {
		return nil
	}
	pending := make(map[string]struct{}, len(targets))
	for _, target := range targets {
		pending[target] = struct{}{}
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		for target := range pending {
			conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(waitCtx, "tcp", target)
			if err != nil {
				continue
			}
			_ = conn.Close()
			delete(pending, target)
		}
		if len(pending) == 0 {
			return nil
		}
		select {
		case <-waitCtx.Done():
			for target := range pending {
				return fmt.Errorf("gost listener %s is not ready: %w", target, waitCtx.Err())
			}
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func serviceListenTargets(services []Service) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(services))
	for _, service := range services {
		target, ok := serviceListenTarget(service.Addr)
		if !ok {
			continue
		}
		if _, exists := seen[target]; exists {
			continue
		}
		seen[target] = struct{}{}
		out = append(out, target)
	}
	return out
}

func serviceListenTarget(addr string) (string, bool) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "", false
	}
	switch host {
	case "", "::", "0.0.0.0", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), true
}

package app

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
)

func (r *Runtime) probeExitIP(ctx context.Context, client *http.Client) (string, error) {
	endpoints := append([]string(nil), r.cfg.ProxyExitGeoURLs...)
	if len(endpoints) == 0 {
		return "", errors.New("proxy exit ip endpoints are not configured")
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type probeResult struct {
		ip  string
		err error
	}
	results := make(chan probeResult, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint := strings.TrimSpace(endpoint)
		if endpoint == "" {
			results <- probeResult{err: errors.New("empty proxy exit ip endpoint")}
			continue
		}
		go func() {
			geo, err := requestIPInfo(probeCtx, client, endpoint, true)
			if err != nil {
				results <- probeResult{err: err}
				return
			}
			if net.ParseIP(geo.IP) == nil {
				results <- probeResult{err: errors.New("proxy exit ip endpoint returned invalid IP")}
				return
			}
			results <- probeResult{ip: geo.IP}
		}()
	}
	for range endpoints {
		select {
		case <-ctx.Done():
			return "", errors.New("check proxy exit ip timed out")
		case result := <-results:
			if result.err == nil && result.ip != "" {
				cancel()
				return result.ip, nil
			}
		}
	}
	return "", errors.New("check proxy exit ip failed")
}

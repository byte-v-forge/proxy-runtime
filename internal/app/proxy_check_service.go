package app

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Runtime) getProxyExitIP(ctx context.Context, req *proxyruntimev1.GetProxyExitIPRequest) (*proxyruntimev1.ProxyExitIP, error) {
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	timeout := proxyExitIPTimeout(settings)
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId(), timeout)
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ip, err := r.probeExitIP(probeCtx, client)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyExitIP{Ip: ip, CheckedAt: timestamppb.Now()}, nil
}

func (r *Runtime) getProxyExitGeo(ctx context.Context, req *proxyruntimev1.GetProxyExitGeoRequest) (*proxyruntimev1.ProxyExitGeo, error) {
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		return nil, errors.New("ip is required")
	}
	geo, err := r.lookupIPGeo(ctx, ip)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyExitGeo{Ip: ip, CountryCode: geo.CountryCode, Region: geo.Region, City: geo.City, CheckedAt: timestamppb.Now()}, nil
}

func (r *Runtime) checkProxyIPFraud(ctx context.Context, req *proxyruntimev1.CheckProxyIPFraudRequest) (*proxyruntimev1.ProxyIPFraudCheck, error) {
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		return nil, errors.New("ip is required")
	}
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	check, err := r.checkIPFraud(ctx, ip, settings)
	if err != nil {
		return nil, errors.New("check IP fraud")
	}
	return check, nil
}

func (r *Runtime) checkProxyEdgeAccess(ctx context.Context, req *proxyruntimev1.CheckProxyEdgeAccessRequest) (*proxyruntimev1.ProxyEdgeAccessCheck, error) {
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	timeout := proxyExitIPTimeout(settings)
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId(), timeout)
	if err != nil {
		return nil, err
	}
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		probeCtx, cancel := context.WithTimeout(ctx, timeout)
		exitIP, err := r.probeExitIP(probeCtx, client)
		cancel()
		if err != nil {
			return nil, err
		}
		ip = exitIP
	}
	outcome := r.runEdgeCanary(ctx, client, settings)
	return buildEdgeAccessCheck(edgeBaseFraudCheck(ip), strings.TrimSpace(req.GetExpectedCountryCode()), outcome), nil
}

func (r *Runtime) checkTargetConnectivity(ctx context.Context, req *proxyruntimev1.CheckProxyTargetConnectivityRequest) (*proxyruntimev1.ProxyTargetConnectivityCheck, error) {
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId(), proxyExitIPTimeout(settings))
	if err != nil {
		return nil, err
	}
	target, err := normalizeConnectivityTarget(req.GetTargetUrl())
	if err != nil {
		return nil, err
	}
	started := time.Now()
	checkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	checkReq.Header.Set("Accept", "text/html,application/json,text/plain;q=0.8")
	checkReq.Header.Set("Cache-Control", "no-cache")
	resp, err := client.Do(checkReq)
	latency := uint32(time.Since(started).Milliseconds())
	out := &proxyruntimev1.ProxyTargetConnectivityCheck{TargetUrl: target, Host: checkReq.URL.Hostname(), LatencyMs: latency, CheckedAt: timestamppb.Now()}
	if err != nil {
		out.ErrorMessage = "target connectivity check failed"
		return out, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	out.StatusCode = uint32(resp.StatusCode)
	out.Reachable = true
	return out, nil
}

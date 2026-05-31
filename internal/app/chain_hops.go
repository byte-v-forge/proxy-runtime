package app

import (
	"context"
	"net"
	"net/url"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

const chainHopEnrichmentTimeout = 2 * time.Second

func (r *Runtime) chainPlanHops(ctx context.Context, line *scoredLineCandidate, gateway scoredGatewayCandidate) []*proxyruntimev1.ProxyChainHop {
	hops := make([]*proxyruntimev1.ProxyChainHop, 0, 2)
	if line != nil && line.proto != nil {
		hop := &proxyruntimev1.ProxyChainHop{
			HopId:             "line:" + line.proto.GetSourceId() + ":" + line.proto.GetNodeId(),
			Order:             uint32(len(hops) + 1),
			Role:              proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_LINE_PROXY,
			SourceKind:        line.proto.GetSourceKind(),
			SourceId:          line.proto.GetSourceId(),
			SourceDisplayName: line.proto.GetSourceDisplayName(),
			NodeId:            line.proto.GetNodeId(),
			NodeDisplayName:   line.proto.GetDisplayName(),
			Status:            line.proto.GetStatus(),
			DelayMs:           line.proto.GetDelayMs(),
		}
		hops = append(hops, r.enrichChainHop(ctx, hop, func(resolveCtx context.Context) (string, error) {
			return r.sourcePlane.ResolveNodePublicIP(resolveCtx, line.proto.GetSourceId(), line.proto.GetNodeId(), line.proto.GetDisplayName())
		}))
	}
	if gateway.proto != nil {
		hop := &proxyruntimev1.ProxyChainHop{
			HopId:              "dynamic-gateway:" + gateway.proto.GetProviderAccountId() + ":" + gateway.proto.GetGatewayId(),
			Order:              uint32(len(hops) + 1),
			Role:               proxyruntimev1.ProxyChainHopRole_PROXY_CHAIN_HOP_ROLE_DYNAMIC_GATEWAY,
			SourceKind:         proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_DYNAMIC_IP,
			ProviderAccountId:  gateway.proto.GetProviderAccountId(),
			ProviderId:         gateway.proto.GetProviderId(),
			GatewayId:          gateway.proto.GetGatewayId(),
			GatewayDisplayName: gateway.proto.GetDisplayName(),
		}
		hops = append(hops, r.enrichChainHop(ctx, hop, func(resolveCtx context.Context) (string, error) {
			return resolvePublicIP(resolveCtx, networkAddressHost(gateway.gateway.Addr))
		}))
	}
	return hops
}

func (r *Runtime) enrichChainHop(ctx context.Context, hop *proxyruntimev1.ProxyChainHop, resolve func(context.Context) (string, error)) *proxyruntimev1.ProxyChainHop {
	if hop == nil || resolve == nil {
		return hop
	}
	enrichCtx, cancel := context.WithTimeout(ctx, chainHopEnrichmentTimeout)
	defer cancel()
	results := make(chan *proxyruntimev1.ProxyChainHop, 1)
	go func() {
		enriched := *hop
		ip, err := resolve(enrichCtx)
		if err != nil {
			r.logger.Warn("resolve proxy chain hop public ip failed", "hop_id", hop.GetHopId(), "error", err)
			results <- &enriched
			return
		}
		r.fillChainHopGeo(enrichCtx, &enriched, ip)
		results <- &enriched
	}()
	select {
	case <-ctx.Done():
		return hop
	case <-enrichCtx.Done():
		return hop
	case enriched := <-results:
		return enriched
	}
}

func (r *Runtime) fillChainHopGeo(ctx context.Context, hop *proxyruntimev1.ProxyChainHop, ip string) {
	if hop == nil {
		return
	}
	ip = strings.TrimSpace(ip)
	if net.ParseIP(ip) == nil {
		return
	}
	hop.ObservedIp = ip
	geo, err := r.lookupIPGeo(ctx, ip)
	if err != nil {
		r.logger.Warn("resolve proxy chain hop geo failed", "hop_id", hop.GetHopId(), "observed_ip", ip, "error", err)
		return
	}
	hop.CountryCode = geo.CountryCode
	hop.Region = geo.Region
	hop.City = geo.City
}

func chainHopByRole(plan *proxyruntimev1.ProxyChainPlan, role proxyruntimev1.ProxyChainHopRole) *proxyruntimev1.ProxyChainHop {
	for _, hop := range plan.GetHops() {
		if hop.GetRole() == role {
			return hop
		}
	}
	return nil
}

func networkAddressHost(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if host, _, err := net.SplitHostPort(addr); err == nil {
		return strings.Trim(host, "[]")
	}
	if parsed, err := url.Parse(addr); err == nil && parsed != nil && parsed.Hostname() != "" {
		return parsed.Hostname()
	}
	return strings.Trim(addr, "[]")
}

func resolvePublicIP(ctx context.Context, host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ip := addr.IP.To4(); ip != nil {
			return ip.String(), nil
		}
	}
	if len(addrs) > 0 {
		return addrs[0].IP.String(), nil
	}
	return "", nil
}

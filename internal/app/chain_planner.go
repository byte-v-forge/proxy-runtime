package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type chainPlanResult struct {
	plan              *proxyruntimev1.ProxyChainPlan
	gateway           accountproxy.Gateway
	lineNode          *provider.Node
	lineCandidates    []*proxyruntimev1.ProxyLineCandidate
	gatewayCandidates []*proxyruntimev1.ProxyDynamicGatewayCandidate
}

type scoredGatewayCandidate struct {
	proto   *proxyruntimev1.ProxyDynamicGatewayCandidate
	gateway accountproxy.Gateway
	score   int
}

type scoredLineCandidate struct {
	proto *proxyruntimev1.ProxyLineCandidate
	score int
}

func (r *Runtime) resolveProxyChain(ctx context.Context, req *proxyruntimev1.ResolveProxyChainRequest) (*proxyruntimev1.ResolveProxyChainResponse, error) {
	acquire := &proxyruntimev1.AcquireProxyLeaseRequest{
		AccountId:   strings.TrimSpace(req.GetAccountId()),
		Policy:      req.GetSessionPolicy(),
		ChainPolicy: req.GetChainPolicy(),
		Purpose:     req.GetChainPolicy().GetPurpose(),
	}
	result, err := r.planProxyChain(ctx, acquire)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ResolveProxyChainResponse{Plan: result.plan, LineCandidates: result.lineCandidates, DynamicGatewayCandidates: result.gatewayCandidates}, nil
}

func (r *Runtime) planProxyChain(ctx context.Context, req *proxyruntimev1.AcquireProxyLeaseRequest) (chainPlanResult, error) {
	settings, err := r.settings.load()
	if err != nil {
		return chainPlanResult{}, err
	}
	policy := normalizeChainPolicy(req)
	gateways, err := r.dynamicGatewayCandidates(ctx, settings, policy)
	if err != nil {
		return chainPlanResult{}, err
	}
	if len(gateways) == 0 {
		return chainPlanResult{}, errors.New("no dynamic IP gateway candidate")
	}
	attempt := chainAttempt(req)
	selectedGateway := chooseGatewayCandidate(gateways, policy, gatewaySelectionKey(req), attempt)
	lines, lineErr := r.lineCandidates(ctx, policy)
	if lineErr != nil {
		r.logger.Warn("proxy line candidate discovery failed", "error", lineErr)
	}
	selectedLine := chooseLineCandidate(lines, policy, req.GetAccountId(), attempt)
	pool := r.currentPool()
	lineNode := r.sourceRuntimeNodeForLine(pool, selectedLineProto(selectedLine))
	reasons := []string{fmt.Sprintf("dynamic_gateway=%s/%s/%s", selectedGateway.proto.GetProviderAccountId(), selectedGateway.proto.GetProviderId(), selectedGateway.proto.GetGatewayId())}
	if selectedLine != nil && lineNode != nil {
		reasons = append(reasons, fmt.Sprintf("line=%s/%s", selectedLine.proto.GetSourceId(), selectedLine.proto.GetNodeId()))
	} else {
		if selectedLine != nil {
			return chainPlanResult{}, fmt.Errorf("selected line proxy listener is not available: %s/%s", selectedLine.proto.GetSourceId(), selectedLine.proto.GetNodeId())
		}
		if !policy.GetAllowDirectDynamicGateway() {
			return chainPlanResult{}, errors.New("no line proxy candidate and direct dynamic gateway is disabled")
		}
		selectedLine = nil
		reasons = append(reasons, "line=direct_dynamic_gateway")
	}
	plan := &proxyruntimev1.ProxyChainPlan{
		ChainId:          "chain-" + shortHash(req.GetAccountId()+":"+policy.GetPurpose()),
		Policy:           policy,
		DynamicGateway:   selectedGateway.proto,
		SelectionReasons: reasons,
		PlannedAt:        timestamppb.New(time.Now().UTC()),
	}
	if selectedLine != nil {
		plan.Line = selectedLine.proto
	}
	plan.Hops = r.chainPlanHops(ctx, selectedLine, selectedGateway)
	return chainPlanResult{plan: plan, gateway: selectedGateway.gateway, lineNode: lineNodeForPlan(plan, lineNode), lineCandidates: lineCandidateProtos(lines), gatewayCandidates: gatewayCandidateProtos(gateways)}, nil
}

func selectedLineProto(candidate *scoredLineCandidate) *proxyruntimev1.ProxyLineCandidate {
	if candidate == nil {
		return nil
	}
	return candidate.proto
}

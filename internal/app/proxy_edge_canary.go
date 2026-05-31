package app

import (
	"context"
	"io"
	"net/http"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

type edgeCanaryOutcome struct {
	level        proxyruntimev1.ProxyEdgeAccessRiskLevel
	score        float64
	signal       proxyruntimev1.ProxyEdgeAccessRiskSignal
	errorMessage string
}

func (r *Runtime) runEdgeCanary(ctx context.Context, client *http.Client, settings *runtimeSettingsFile) edgeCanaryOutcome {
	edgeCanary := settings.GetEdgeCanary()
	target := strings.TrimSpace(edgeCanary.GetUrl())
	token := strings.TrimSpace(edgeCanary.GetToken())
	if !edgeCanaryEnabled(edgeCanary) || target == "" || token == "" {
		return edgeCanaryOutcome{
			level:        proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNSUPPORTED,
			signal:       proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_ACCESS_UNSUPPORTED,
			errorMessage: "edge access canary is not configured",
		}
	}
	canaryCtx, cancel := context.WithTimeout(ctx, r.cfg.EdgeCanaryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(canaryCtx, http.MethodGet, target, nil)
	if err != nil {
		return edgeUnavailableOutcome()
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("X-Canary-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		return edgeUnavailableOutcome()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return edgeUnavailableOutcome()
	}
	if edgeChallengeDetected(resp.Header, body) {
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_CHALLENGE_LIKELY,
			score:  85,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_CHALLENGE_DETECTED,
		}
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH,
			score:  70,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_RATE_LIMIT_DETECTED,
		}
	case resp.StatusCode == http.StatusForbidden:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_BLOCK_LIKELY,
			score:  90,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_BLOCK_DETECTED,
		}
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized:
		return edgeUnavailableOutcome()
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW,
			score:  0,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_ACCESS_PASSED,
		}
	default:
		return edgeUnavailableOutcome()
	}
}

func edgeUnavailableOutcome() edgeCanaryOutcome {
	return edgeCanaryOutcome{
		level:        proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN,
		signal:       proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_UNAVAILABLE,
		errorMessage: "edge access check unavailable",
	}
}

func edgeChallengeDetected(header http.Header, body []byte) bool {
	if strings.EqualFold(strings.TrimSpace(header.Get("cf-mitigated")), "challenge") {
		return true
	}
	text := strings.ToLower(string(body))
	for _, hint := range []string{"challenge-platform", "cf-chl", "just a moment"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

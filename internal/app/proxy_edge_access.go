package app

import (
	"sort"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func buildEdgeAccessCheck(
	fraudCheck *proxyruntimev1.ProxyIPFraudCheck,
	expectedCountryCode string,
	outcome edgeCanaryOutcome,
) *proxyruntimev1.ProxyEdgeAccessCheck {
	check := &proxyruntimev1.ProxyEdgeAccessCheck{
		Ip:           fraudCheck.GetIp(),
		IpFraudCheck: fraudCheck,
		RiskLevel:    proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN,
		RiskScore:    clampEdgeScore(fraudCheck.GetRiskScore()),
		CheckedAt:    fraudCheck.GetCheckedAt(),
		ErrorMessage: outcome.errorMessage,
	}
	if outcome.level == proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNSUPPORTED {
		check.RiskLevel = outcome.level
		check.RiskSignals = []proxyruntimev1.ProxyEdgeAccessRiskSignal{outcome.signal}
		return check
	}
	signals := map[proxyruntimev1.ProxyEdgeAccessRiskSignal]struct{}{}
	addSignal := func(signal proxyruntimev1.ProxyEdgeAccessRiskSignal) {
		if signal != proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_UNSPECIFIED {
			signals[signal] = struct{}{}
		}
	}
	for _, signal := range fraudCheck.GetRiskSignals() {
		switch signal {
		case proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_DATACENTER,
			proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_HOSTING:
			addSignal(proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_DATACENTER_NETWORK)
		case proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_PROXY,
			proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_VPN,
			proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_TOR:
			addSignal(proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_ANONYMIZER_DETECTED)
		}
	}
	if fraudCheck.GetRiskLevel() == proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_HIGH ||
		fraudCheck.GetRiskLevel() == proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_CRITICAL {
		addSignal(proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_IP_REPUTATION_RISK)
	}
	if countryMismatch(fraudCheck.GetCountryCode(), expectedCountryCode) {
		addSignal(proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_GEO_MISMATCH)
		check.RiskScore = maxFloat(check.GetRiskScore(), 60)
	}
	addSignal(outcome.signal)
	check.RiskScore = maxFloat(check.GetRiskScore(), outcome.score)
	check.RiskLevel = edgeRiskFromScore(check.GetRiskScore())
	check.RiskLevel = maxEdgeRisk(check.GetRiskLevel(), edgeRiskFromIP(fraudCheck.GetRiskLevel()))
	check.RiskLevel = maxEdgeRisk(check.GetRiskLevel(), outcome.level)
	if outcome.signal == proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_UNAVAILABLE &&
		check.GetRiskLevel() == proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW {
		check.RiskLevel = proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN
	}
	check.RiskSignals = sortedEdgeSignals(signals)
	return check
}

func countryMismatch(actual string, expected string) bool {
	actual = strings.ToUpper(strings.TrimSpace(actual))
	expected = strings.ToUpper(strings.TrimSpace(expected))
	return actual != "" && expected != "" && actual != expected
}

func edgeRiskFromIP(level proxyruntimev1.ProxyIPFraudRiskLevel) proxyruntimev1.ProxyEdgeAccessRiskLevel {
	switch level {
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_CRITICAL:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_HIGH:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_MEDIUM:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_MEDIUM
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW
	default:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN
	}
}

func edgeRiskFromScore(score float64) proxyruntimev1.ProxyEdgeAccessRiskLevel {
	switch {
	case score >= 80:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH
	case score >= 50:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_MEDIUM
	case score > 0:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW
	default:
		return proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN
	}
}

func maxEdgeRisk(
	current proxyruntimev1.ProxyEdgeAccessRiskLevel,
	next proxyruntimev1.ProxyEdgeAccessRiskLevel,
) proxyruntimev1.ProxyEdgeAccessRiskLevel {
	if edgeRiskRank(next) > edgeRiskRank(current) {
		return next
	}
	return current
}

func edgeRiskRank(level proxyruntimev1.ProxyEdgeAccessRiskLevel) int {
	switch level {
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_BLOCK_LIKELY:
		return 70
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_CHALLENGE_LIKELY:
		return 60
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH:
		return 50
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_MEDIUM:
		return 40
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW:
		return 30
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN:
		return 20
	case proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNSUPPORTED:
		return 5
	default:
		return 0
	}
}

func sortedEdgeSignals(values map[proxyruntimev1.ProxyEdgeAccessRiskSignal]struct{}) []proxyruntimev1.ProxyEdgeAccessRiskSignal {
	signals := make([]proxyruntimev1.ProxyEdgeAccessRiskSignal, 0, len(values))
	for signal := range values {
		signals = append(signals, signal)
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i] < signals[j] })
	return signals
}

func clampEdgeScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

func maxFloat(left float64, right float64) float64 {
	if right > left {
		return right
	}
	return left
}

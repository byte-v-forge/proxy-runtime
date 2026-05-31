package ipfraud

import (
	"sort"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func mergeReports(ip string, reports []report, errorMessage string) *proxyruntimev1.ProxyIPFraudCheck {
	check := &proxyruntimev1.ProxyIPFraudCheck{
		Ip:             ip,
		NetworkKind:    proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_UNKNOWN,
		AnonymizerKind: proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_UNKNOWN,
		RiskLevel:      proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNKNOWN,
		CheckedAt:      timestamppb.Now(),
		ErrorMessage:   errorMessage,
	}
	if len(reports) == 0 {
		return check
	}
	signals := map[proxyruntimev1.ProxyIPFraudSignal]struct{}{}
	for _, item := range reports {
		check.NetworkKind = chooseNetworkKind(check.GetNetworkKind(), item.networkKind)
		check.AnonymizerKind = chooseAnonymizerKind(check.GetAnonymizerKind(), item.anonymizerKind)
		nextRiskLevel := normalizeRiskLevel(item.riskLevel, item.riskScore)
		if riskRank(nextRiskLevel) > riskRank(check.GetRiskLevel()) {
			check.ProviderId = item.providerID
			check.ProviderDisplayName = item.providerName
		}
		check.RiskLevel = chooseRiskLevel(check.GetRiskLevel(), nextRiskLevel)
		if item.riskScore > check.RiskScore {
			check.RiskScore = clampScore(item.riskScore)
			check.ProviderId = item.providerID
			check.ProviderDisplayName = item.providerName
		}
		for _, signal := range item.signals {
			if signal != proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_UNSPECIFIED {
				signals[signal] = struct{}{}
			}
		}
		assignFirst(&check.CountryCode, item.countryCode)
		assignFirst(&check.Region, item.region)
		assignFirst(&check.City, item.city)
		assignFirst(&check.Asn, item.asn)
		assignFirst(&check.Organization, item.organization)
		assignFirst(&check.Isp, item.isp)
	}
	if strings.TrimSpace(check.GetProviderId()) == "" {
		for _, item := range reports {
			assignFirst(&check.ProviderId, item.providerID)
			assignFirst(&check.ProviderDisplayName, item.providerName)
			if check.GetProviderId() != "" || check.GetProviderDisplayName() != "" {
				break
			}
		}
	}
	check.RiskSignals = sortedSignals(signals)
	if check.GetAnonymizerKind() == proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_UNKNOWN && len(check.GetRiskSignals()) == 0 {
		check.AnonymizerKind = proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_NONE
	}
	if check.GetRiskLevel() == proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNKNOWN && check.GetRiskScore() == 0 {
		check.RiskLevel = proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW
	}
	return check
}

func assignFirst(target *string, value string) {
	if strings.TrimSpace(*target) == "" {
		*target = strings.TrimSpace(value)
	}
}

func sortedSignals(values map[proxyruntimev1.ProxyIPFraudSignal]struct{}) []proxyruntimev1.ProxyIPFraudSignal {
	signals := make([]proxyruntimev1.ProxyIPFraudSignal, 0, len(values))
	for signal := range values {
		signals = append(signals, signal)
	}
	sort.Slice(signals, func(i, j int) bool { return signals[i] < signals[j] })
	return signals
}

func chooseNetworkKind(current, next proxyruntimev1.ProxyIPNetworkKind) proxyruntimev1.ProxyIPNetworkKind {
	if networkRank(next) > networkRank(current) {
		return next
	}
	return current
}

func networkRank(value proxyruntimev1.ProxyIPNetworkKind) int {
	switch value {
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_DATACENTER:
		return 80
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BROADCAST:
		return 70
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ANYCAST:
		return 65
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_SATELLITE:
		return 60
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_MOBILE:
		return 50
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BUSINESS:
		return 40
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ISP:
		return 35
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_RESIDENTIAL:
		return 30
	case proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_UNKNOWN:
		return 10
	default:
		return 0
	}
}

func chooseAnonymizerKind(current, next proxyruntimev1.ProxyIPAnonymizerKind) proxyruntimev1.ProxyIPAnonymizerKind {
	if anonymizerRank(next) > anonymizerRank(current) {
		return next
	}
	return current
}

func anonymizerRank(value proxyruntimev1.ProxyIPAnonymizerKind) int {
	switch value {
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_TOR:
		return 80
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_VPN:
		return 70
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_PROXY:
		return 60
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_CRAWLER:
		return 50
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_NONE:
		return 20
	case proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_UNKNOWN:
		return 10
	default:
		return 0
	}
}

func chooseRiskLevel(current, next proxyruntimev1.ProxyIPFraudRiskLevel) proxyruntimev1.ProxyIPFraudRiskLevel {
	if riskRank(next) > riskRank(current) {
		return next
	}
	return current
}

func normalizeRiskLevel(level proxyruntimev1.ProxyIPFraudRiskLevel, score float64) proxyruntimev1.ProxyIPFraudRiskLevel {
	if level != proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNSPECIFIED &&
		level != proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNKNOWN {
		return level
	}
	return riskLevelFromScore(score)
}

func riskLevelFromScore(score float64) proxyruntimev1.ProxyIPFraudRiskLevel {
	switch {
	case score >= 90:
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_CRITICAL
	case score >= 70:
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_HIGH
	case score >= 40:
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_MEDIUM
	default:
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW
	}
}

func riskRank(value proxyruntimev1.ProxyIPFraudRiskLevel) int {
	switch value {
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_CRITICAL:
		return 50
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_HIGH:
		return 40
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_MEDIUM:
		return 30
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW:
		return 20
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNKNOWN:
		return 10
	case proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNSUPPORTED:
		return 5
	default:
		return 0
	}
}

func clampScore(score float64) float64 {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}

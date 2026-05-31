package ipfraud

import (
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func classifyNetworkKind(values ...string) proxyruntimev1.ProxyIPNetworkKind {
	value := strings.ToLower(strings.Join(values, " "))
	switch {
	case strings.Contains(value, "datacenter") || strings.Contains(value, "data center") || strings.Contains(value, "hosting") || strings.Contains(value, "hosted"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_DATACENTER
	case strings.Contains(value, "mobile") || strings.Contains(value, "cellular"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_MOBILE
	case strings.Contains(value, "residential") || strings.Contains(value, "consumer"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_RESIDENTIAL
	case strings.Contains(value, "satellite"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_SATELLITE
	case strings.Contains(value, "broadcast"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BROADCAST
	case strings.Contains(value, "anycast"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ANYCAST
	case strings.Contains(value, "business") || strings.Contains(value, "enterprise") || strings.Contains(value, "corporate") ||
		strings.Contains(value, "commercial") || strings.Contains(value, "organization") || strings.Contains(value, "government") ||
		strings.Contains(value, "military") || strings.Contains(value, "university") || strings.Contains(value, "library"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BUSINESS
	case strings.Contains(value, "isp") || strings.Contains(value, "fixed line") || strings.Contains(value, "cable") || strings.Contains(value, "dsl"):
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ISP
	default:
		return proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_UNKNOWN
	}
}

func classifyAnonymizerKind(tor, vpn, proxy, crawler bool) proxyruntimev1.ProxyIPAnonymizerKind {
	switch {
	case tor:
		return proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_TOR
	case vpn:
		return proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_VPN
	case proxy:
		return proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_PROXY
	case crawler:
		return proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_CRAWLER
	default:
		return proxyruntimev1.ProxyIPAnonymizerKind_PROXY_IP_ANONYMIZER_KIND_NONE
	}
}

func riskLevelFromText(value string) proxyruntimev1.ProxyIPFraudRiskLevel {
	switch compactLower(value) {
	case "critical", "very_high", "very high", "severe":
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_CRITICAL
	case "high", "risky":
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_HIGH
	case "medium", "moderate":
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_MEDIUM
	case "low", "safe", "clean":
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW
	default:
		return proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNKNOWN
	}
}

func signalsFromFlags(flags riskFlags) []proxyruntimev1.ProxyIPFraudSignal {
	signals := []proxyruntimev1.ProxyIPFraudSignal{}
	add := func(enabled bool, signal proxyruntimev1.ProxyIPFraudSignal) {
		if enabled {
			signals = append(signals, signal)
		}
	}
	add(flags.bogon, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_BOGON)
	add(flags.datacenter, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_DATACENTER)
	add(flags.hosting, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_HOSTING)
	add(flags.proxy, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_PROXY)
	add(flags.vpn, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_VPN)
	add(flags.tor, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_TOR)
	add(flags.abuser, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_ABUSER)
	add(flags.crawler, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_CRAWLER)
	add(flags.mobile, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_MOBILE)
	add(flags.satellite, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_SATELLITE)
	add(flags.broadcast, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_BROADCAST)
	add(flags.anycast, proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_ANYCAST)
	return signals
}

func scoreFromFlags(flags riskFlags) float64 {
	score := 0.0
	max := func(value float64) {
		if value > score {
			score = value
		}
	}
	maxIf := func(enabled bool, value float64) {
		if enabled {
			max(value)
		}
	}
	maxIf(flags.bogon, 95)
	maxIf(flags.tor, 90)
	maxIf(flags.abuser, 85)
	maxIf(flags.vpn, 75)
	maxIf(flags.proxy, 70)
	maxIf(flags.crawler, 60)
	maxIf(flags.datacenter || flags.hosting, 55)
	maxIf(flags.broadcast || flags.anycast, 50)
	maxIf(flags.satellite, 45)
	maxIf(flags.mobile, 15)
	return score
}

func networkKindWithFlags(base proxyruntimev1.ProxyIPNetworkKind, flags riskFlags) proxyruntimev1.ProxyIPNetworkKind {
	networkKind := base
	if flags.datacenter || flags.hosting {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_DATACENTER)
	}
	if flags.mobile {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_MOBILE)
	}
	if flags.satellite {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_SATELLITE)
	}
	if flags.broadcast {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_BROADCAST)
	}
	if flags.anycast {
		networkKind = chooseNetworkKind(networkKind, proxyruntimev1.ProxyIPNetworkKind_PROXY_IP_NETWORK_KIND_ANYCAST)
	}
	return networkKind
}

func hasNonEmptyString(value map[string]any, paths ...string) bool {
	for _, path := range paths {
		if strings.TrimSpace(stringValue(value, path)) != "" {
			return true
		}
	}
	return false
}

type riskFlags struct {
	bogon      bool
	datacenter bool
	hosting    bool
	proxy      bool
	vpn        bool
	tor        bool
	abuser     bool
	crawler    bool
	mobile     bool
	satellite  bool
	broadcast  bool
	anycast    bool
}

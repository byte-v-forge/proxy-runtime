package app

import (
	"context"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (r *Runtime) checkIPFraud(ctx context.Context, ip string, settings *runtimeSettingsFile) (*proxyruntimev1.ProxyIPFraudCheck, error) {
	providers := ipFraudProviders(settings)
	if len(providers) == 0 {
		return unsupportedIPFraudCheck(ip), nil
	}
	service := r.ipFraudChecker(settings, providers)
	return service.Check(ctx, ip)
}

func (r *Runtime) ipFraudChecker(settings *runtimeSettingsFile, providers []ipfraud.ProviderConfig) ipFraudChecker {
	signature := runtimeSettingsSignature(settings)
	r.fraudMu.Lock()
	defer r.fraudMu.Unlock()
	if r.fraud != nil && r.fraudSignature == signature {
		return r.fraud
	}
	r.fraud = newIPFraudChecker(r.cfg.IPFraud, providers, r.logger)
	r.fraudSignature = signature
	return r.fraud
}

func (r *Runtime) resetIPFraudChecker() {
	r.fraudMu.Lock()
	defer r.fraudMu.Unlock()
	r.fraud = nil
	r.fraudSignature = ""
}

func edgeBaseFraudCheck(ip string) *proxyruntimev1.ProxyIPFraudCheck {
	return &proxyruntimev1.ProxyIPFraudCheck{
		Ip:        ip,
		RiskLevel: proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_LOW,
		CheckedAt: timestamppb.Now(),
	}
}

func unsupportedIPFraudCheck(ip string) *proxyruntimev1.ProxyIPFraudCheck {
	return &proxyruntimev1.ProxyIPFraudCheck{
		Ip:        ip,
		RiskLevel: proxyruntimev1.ProxyIPFraudRiskLevel_PROXY_IP_FRAUD_RISK_LEVEL_UNSUPPORTED,
		RiskSignals: []proxyruntimev1.ProxyIPFraudSignal{
			proxyruntimev1.ProxyIPFraudSignal_PROXY_IP_FRAUD_SIGNAL_FRAUD_CHECK_UNSUPPORTED,
		},
		CheckedAt:    timestamppb.Now(),
		ErrorMessage: "IP fraud check is not configured",
	}
}

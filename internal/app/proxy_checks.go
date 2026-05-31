package app

import (
	"context"
	"log/slog"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
)

type ipFraudChecker interface {
	Check(ctx context.Context, ip string) (*proxyruntimev1.ProxyIPFraudCheck, error)
}

type proxyExitGeo struct {
	IP          string
	CountryCode string
	Region      string
	City        string
}

type cachedIPGeo struct {
	geo       proxyExitGeo
	expiresAt time.Time
}

const ipGeoCacheTTL = 24 * time.Hour

func newIPFraudChecker(cfg config.IPFraudConfig, providers []ipfraud.ProviderConfig, logger *slog.Logger) ipFraudChecker {
	return ipfraud.NewService(ipfraud.Config{
		Providers:   providers,
		Timeout:     cfg.Timeout,
		CacheTTL:    cfg.CacheTTL,
		KeyCooldown: cfg.KeyCooldown,
	}, logger)
}

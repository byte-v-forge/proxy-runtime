package ten24

import (
	"net/http"
)

const (
	providerID           = "1024proxy"
	defaultStickyMinutes = 30
	minStickyMinutes     = 1
	maxStickyMinutes     = 120
)

type Provider struct {
	cfg        Config
	httpClient *http.Client
}

func New(cfg Config, httpClient *http.Client) *Provider {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Provider{cfg: cfg, httpClient: httpClient}
}

func (p *Provider) Name() string {
	return providerID
}

func (p *Provider) RequiresSessionLease() bool {
	return true
}

func (p *Provider) supportsCredentialSession() bool {
	return p.cfg.ProxyAddr != "" && p.cfg.Username != "" && p.cfg.Password != ""
}

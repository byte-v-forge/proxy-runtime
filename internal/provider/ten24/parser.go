package ten24

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

func (p *Provider) parseAPIResponse(body []byte) ([]provider.Node, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, nil
	}
	if trimmed[0] == '{' || trimmed[0] == '[' || p.cfg.APIType == "json" {
		return p.parseJSON(trimmed)
	}
	return p.parseText(string(trimmed))
}

func (p *Provider) parseJSON(body []byte) ([]provider.Node, error) {
	var payload any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("parse 1024proxy JSON response: %w", err)
	}
	rawValues := proxyurl.Collect(payload)
	return p.parseRawValues(rawValues)
}

func (p *Provider) parseText(body string) ([]provider.Node, error) {
	rawValues := strings.FieldsFunc(body, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	return p.parseRawValues(rawValues)
}

func (p *Provider) parseRawValues(rawValues []string) ([]provider.Node, error) {
	nodes := make([]provider.Node, 0, len(rawValues))
	for _, raw := range rawValues {
		proxyURL, err := proxyurl.Parse(raw, defaultProtocol(p.cfg.Protocol))
		if err != nil {
			continue
		}
		nodes = append(nodes, provider.Node{
			ID:           fmt.Sprintf("1024proxy-api-%d", len(nodes)),
			URL:          proxyURL,
			ProviderID:   p.Name(),
			UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_PROXY_POOL,
			RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST,
			Labels: map[string]string{
				"mode": "api",
			},
		})
	}
	return nodes, nil
}

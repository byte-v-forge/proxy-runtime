package ten24

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/byte-v-forge/common-lib/httpx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

func (p *Provider) fetchAPI(ctx context.Context) ([]provider.Node, error) {
	apiURL, err := p.buildAPIURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build 1024proxy API request: %w", err)
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call 1024proxy API: %w", err)
	}
	defer resp.Body.Close()

	body, err := httpx.ReadLimited(resp.Body, 1<<20)
	if err != nil {
		return nil, fmt.Errorf("read 1024proxy API response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("1024proxy API returned status %d", resp.StatusCode)
	}

	nodes, err := p.parseAPIResponse(body)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("1024proxy API returned no proxies")
	}
	return nodes, nil
}

func (p *Provider) buildAPIURL() (string, error) {
	parsed, err := url.Parse(p.cfg.APIURL)
	if err != nil {
		return "", errors.New("invalid 1024proxy API URL")
	}
	query := parsed.Query()
	setQuery(query, "region", p.cfg.APIRegion)
	setQuery(query, "format", p.cfg.APIFormat)
	setQuery(query, "time", p.cfg.APITime)
	setQuery(query, "num", p.cfg.APINum)
	setQuery(query, "type", p.cfg.APIType)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

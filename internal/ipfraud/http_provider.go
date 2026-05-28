package ipfraud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type httpProvider struct {
	client   *http.Client
	template string
	auth     AuthConfig
	keys     *keyRing
}

func newHTTPProvider(client *http.Client, template string, auth AuthConfig, cooldown time.Duration) httpProvider {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	if cooldown <= 0 {
		cooldown = 24 * time.Hour
	}
	keys := []string{}
	if auth.APIKey != nil {
		keys = append(keys, auth.APIKey.Keys...)
	}
	return httpProvider{
		client:   client,
		template: template,
		auth:     auth,
		keys:     newKeyRing(keys, cooldown),
	}
}

func (p *httpProvider) lookupJSON(ctx context.Context, ip string) (map[string]any, error) {
	attempts := p.keys.size()
	if attempts == 0 {
		attempts = 1
	}
	for attempt := 0; attempt < attempts; attempt++ {
		keyIndex, key, ok := p.keys.nextAvailable(time.Now())
		if !ok {
			return nil, errQuotaExhausted
		}
		payload, retryAfter, err := p.requestJSON(ctx, ip, key)
		if err == nil {
			return payload, nil
		}
		if errors.Is(err, errQuotaExhausted) {
			p.keys.markUnavailable(keyIndex, retryAfter)
			continue
		}
		return nil, err
	}
	return nil, errQuotaExhausted
}

func (p *httpProvider) requestJSON(ctx context.Context, ip string, key string) (map[string]any, time.Duration, error) {
	rawURL, err := p.requestURL(ip, key)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Accept", "application/json")
	p.applyAuth(req, key)
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, 0, err
	}
	if retryAfter, quota := quotaResponse(resp.StatusCode, resp.Header, body); quota {
		return nil, retryAfter, quotaError{retryAfter: retryAfter}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, 0, fmt.Errorf("IP fraud request failed")
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, 0, fmt.Errorf("decode IP fraud response")
	}
	if quotaPayload(payload) {
		return nil, 0, quotaError{}
	}
	return payload, 0, nil
}

func (p *httpProvider) requestURL(ip string, key string) (string, error) {
	value := strings.ReplaceAll(p.template, "{ip}", url.QueryEscape(ip))
	value = strings.ReplaceAll(value, "{key}", url.QueryEscape(key))
	if p.auth.APIKey == nil || p.auth.APIKey.Placement != "query" || key == "" || strings.Contains(p.template, "{key}") {
		return value, nil
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	query.Set(p.auth.APIKey.Name, key)
	parsed.RawQuery = query.Encode()
	return parsed.String(), nil
}

func (p *httpProvider) applyAuth(req *http.Request, key string) {
	if p.auth.APIKey == nil || key == "" {
		return
	}
	switch p.auth.APIKey.Placement {
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+key)
	case "header":
		req.Header.Set(p.auth.APIKey.Name, key)
	}
}

func quotaResponse(status int, header http.Header, body []byte) (time.Duration, bool) {
	retryAfter := retryAfterDuration(header.Get("Retry-After"))
	if status == http.StatusTooManyRequests || status == http.StatusPaymentRequired {
		return retryAfter, true
	}
	for _, name := range []string{"X-RateLimit-Remaining", "RateLimit-Remaining", "X-Quota-Remaining"} {
		if strings.TrimSpace(header.Get(name)) == "0" {
			return retryAfter, true
		}
	}
	text := strings.ToLower(string(body))
	if status >= 400 {
		for _, hint := range []string{"quota", "limit", "rate", "exceed", "credit"} {
			if strings.Contains(text, hint) {
				return retryAfter, true
			}
		}
	}
	return 0, false
}

func quotaPayload(payload map[string]any) bool {
	text := strings.ToLower(strings.Join([]string{
		stringValue(payload, "error"),
		stringValue(payload, "message"),
		stringValue(payload, "reason"),
		stringValue(payload, "status"),
	}, " "))
	if text == "" {
		return false
	}
	for _, hint := range []string{"quota", "limit", "rate", "exceed", "credit"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

func retryAfterDuration(value string) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(value); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	if at, err := http.ParseTime(value); err == nil {
		return time.Until(at)
	}
	return 0
}

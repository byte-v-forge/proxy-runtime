package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func requestIPInfo(ctx context.Context, client *http.Client, endpoint string, requireIP bool) (proxyExitGeo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return proxyExitGeo{}, err
	}
	req.Header.Set("Accept", "application/json, text/plain;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return proxyExitGeo{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return proxyExitGeo{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return proxyExitGeo{}, errors.New("ip info endpoint unavailable")
	}
	geo := parseIPInfo(body)
	if requireIP && net.ParseIP(geo.IP) == nil {
		return proxyExitGeo{}, errors.New("ip info endpoint returned invalid IP")
	}
	return geo, nil
}

func parseIPInfo(body []byte) proxyExitGeo {
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		ip := jsonString(payload, "ip", "query", "origin")
		if strings.Contains(ip, ",") {
			ip = strings.TrimSpace(strings.Split(ip, ",")[0])
		}
		return proxyExitGeo{
			IP:          ip,
			CountryCode: jsonString(payload, "country_code", "country", "loc"),
			Region:      jsonString(payload, "region", "region_code", "region_name", "state"),
			City:        jsonString(payload, "city"),
		}
	}
	values := map[string]string{}
	for _, line := range strings.Split(string(body), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = strings.TrimSpace(value)
	}
	if ip := values["ip"]; ip != "" {
		return proxyExitGeo{
			IP:          ip,
			CountryCode: values["loc"],
			Region:      firstNonEmpty(values["region"], values["region_name"], values["state"]),
			City:        values["city"],
		}
	}
	return proxyExitGeo{IP: strings.TrimSpace(string(body))}
}

func ipGeoLookupEndpoints(ip string) []string {
	escaped := url.PathEscape(ip)
	return []string{
		"https://ipwho.is/" + escaped,
		"https://ipapi.co/" + escaped + "/json/",
	}
}

func jsonString(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok {
			if text, ok := value.(string); ok {
				return strings.TrimSpace(text)
			}
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return ""
}

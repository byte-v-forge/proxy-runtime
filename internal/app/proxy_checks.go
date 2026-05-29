package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/httpclient"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
	"google.golang.org/protobuf/types/known/timestamppb"
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

func (r *Runtime) handleGetProxyExitIP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.GetProxyExitIPRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	exitIP, err := r.getProxyExitIP(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyExitIPResponse{ProxyExitIp: exitIP})
}

func (r *Runtime) handleGetProxyExitGeo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.GetProxyExitGeoRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	geo, err := r.getProxyExitGeo(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyExitGeoResponse{ProxyExitGeo: geo})
}

func (r *Runtime) handleCheckIPFraud(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyIPFraudRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkProxyIPFraud(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyIPFraudResponse{Check: check})
}

func (r *Runtime) handleCheckEdgeAccessRisk(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyEdgeAccessRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkProxyEdgeAccess(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyEdgeAccessResponse{Check: check})
}

func (r *Runtime) handleCheckTargetConnectivity(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyTargetConnectivityRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkTargetConnectivity(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyTargetConnectivityResponse{Check: check})
}

func (r *Runtime) handleIPFraudProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyIPFraudProvidersResponse{Providers: ipfraud.ProviderDescriptors()})
}

func (r *Runtime) handleRuntimeSettings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		settings, err := r.settings.view()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.writeProto(w, &proxyruntimev1.GetProxyRuntimeSettingsResponse{Settings: settings})
	case http.MethodPost, http.MethodPut:
		var updateReq proxyruntimev1.UpdateProxyRuntimeSettingsRequest
		if req.Body == nil {
			http.Error(w, "request body is required", http.StatusBadRequest)
			return
		}
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &updateReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		settings, err := r.settings.update(&updateReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.resetIPFraudChecker()
		r.writeProto(w, &proxyruntimev1.UpdateProxyRuntimeSettingsResponse{Settings: settings})
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost+", "+http.MethodPut)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (r *Runtime) getProxyExitIP(ctx context.Context, req *proxyruntimev1.GetProxyExitIPRequest) (*proxyruntimev1.ProxyExitIP, error) {
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId())
	if err != nil {
		return nil, err
	}
	probeCtx, cancel := context.WithTimeout(ctx, r.cfg.ProxyExitGeoTimeout)
	defer cancel()
	ip, err := r.probeExitIP(probeCtx, client)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyExitIP{Ip: ip, CheckedAt: timestamppb.Now()}, nil
}

func (r *Runtime) getProxyExitGeo(ctx context.Context, req *proxyruntimev1.GetProxyExitGeoRequest) (*proxyruntimev1.ProxyExitGeo, error) {
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		return nil, errors.New("ip is required")
	}
	geo, err := r.lookupIPGeo(ctx, ip)
	if err != nil {
		return nil, err
	}
	return &proxyruntimev1.ProxyExitGeo{Ip: ip, CountryCode: geo.CountryCode, Region: geo.Region, City: geo.City, CheckedAt: timestamppb.Now()}, nil
}

func (r *Runtime) checkProxyIPFraud(ctx context.Context, req *proxyruntimev1.CheckProxyIPFraudRequest) (*proxyruntimev1.ProxyIPFraudCheck, error) {
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		return nil, errors.New("ip is required")
	}
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	check, err := r.checkIPFraud(ctx, ip, settings)
	if err != nil {
		return nil, errors.New("check IP fraud")
	}
	return check, nil
}

func (r *Runtime) checkProxyEdgeAccess(ctx context.Context, req *proxyruntimev1.CheckProxyEdgeAccessRequest) (*proxyruntimev1.ProxyEdgeAccessCheck, error) {
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId())
	if err != nil {
		return nil, err
	}
	ip := strings.TrimSpace(req.GetIp())
	if net.ParseIP(ip) == nil {
		probeCtx, cancel := context.WithTimeout(ctx, r.cfg.ProxyExitGeoTimeout)
		exitIP, err := r.probeExitIP(probeCtx, client)
		cancel()
		if err != nil {
			return nil, err
		}
		ip = exitIP
	}
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	outcome := r.runEdgeCanary(ctx, client, settings)
	return buildEdgeAccessCheck(edgeBaseFraudCheck(ip), strings.TrimSpace(req.GetExpectedCountryCode()), outcome), nil
}

func (r *Runtime) checkTargetConnectivity(ctx context.Context, req *proxyruntimev1.CheckProxyTargetConnectivityRequest) (*proxyruntimev1.ProxyTargetConnectivityCheck, error) {
	client, err := r.checkProxyHTTPClient(req.GetPoolId(), req.GetProviderId(), req.GetListenerId())
	if err != nil {
		return nil, err
	}
	target, err := normalizeConnectivityTarget(req.GetTargetUrl())
	if err != nil {
		return nil, err
	}
	started := time.Now()
	checkReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	checkReq.Header.Set("Accept", "text/html,application/json,text/plain;q=0.8")
	checkReq.Header.Set("Cache-Control", "no-cache")
	resp, err := client.Do(checkReq)
	latency := uint32(time.Since(started).Milliseconds())
	out := &proxyruntimev1.ProxyTargetConnectivityCheck{TargetUrl: target, Host: checkReq.URL.Hostname(), LatencyMs: latency, CheckedAt: timestamppb.Now()}
	if err != nil {
		out.ErrorMessage = "target connectivity check failed"
		return out, nil
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	out.StatusCode = uint32(resp.StatusCode)
	out.Reachable = true
	return out, nil
}

func (r *Runtime) checkProxyHTTPClient(poolID string, providerID string, listenerID string) (*http.Client, error) {
	if poolID := strings.TrimSpace(poolID); poolID != "" && poolID != "default" {
		return nil, fmt.Errorf("pool %q is not configured", poolID)
	}
	_ = strings.TrimSpace(providerID)
	listener, err := r.checkIPListener(strings.TrimSpace(listenerID))
	if err != nil {
		return nil, err
	}
	proxyURL, err := localProxyURL(listener, r.cfg.LocalProtocol)
	if err != nil {
		return nil, err
	}
	client, err := httpclient.NewWithSchemes(r.cfg.ProxyExitGeoTimeout, proxyURL, httpclient.CommonProxySchemes...)
	if err != nil {
		return nil, errors.New("build IP check client")
	}
	return client, nil
}

func localProxyURL(listener config.EgressListener, defaultProtocol string) (string, error) {
	hostPort, err := localListenHostPort(listener.Addr)
	if err != nil {
		return "", err
	}
	proxy := &url.URL{
		Scheme: listenerProtocol(listener, defaultProtocol),
		Host:   hostPort,
	}
	if listener.Username != "" || listener.Password != "" {
		proxy.User = url.UserPassword(listener.Username, listener.Password)
	}
	return proxy.String(), nil
}

func localListenHostPort(addr string) (string, error) {
	addr = strings.TrimSpace(addr)
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr, nil
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid listener address")
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port), nil
}

func (r *Runtime) probeExitIP(ctx context.Context, client *http.Client) (string, error) {
	endpoints := append([]string(nil), r.cfg.ProxyExitGeoURLs...)
	if len(endpoints) == 0 {
		return "", errors.New("proxy exit ip endpoints are not configured")
	}
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	type probeResult struct {
		ip  string
		err error
	}
	results := make(chan probeResult, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint := strings.TrimSpace(endpoint)
		if endpoint == "" {
			results <- probeResult{err: errors.New("empty proxy exit ip endpoint")}
			continue
		}
		go func() {
			geo, err := requestIPInfo(probeCtx, client, endpoint, true)
			if err != nil {
				results <- probeResult{err: err}
				return
			}
			if net.ParseIP(geo.IP) == nil {
				results <- probeResult{err: errors.New("proxy exit ip endpoint returned invalid IP")}
				return
			}
			results <- probeResult{ip: geo.IP}
		}()
	}
	for range endpoints {
		select {
		case <-ctx.Done():
			return "", errors.New("check proxy exit ip timed out")
		case result := <-results:
			if result.err == nil && result.ip != "" {
				cancel()
				return result.ip, nil
			}
		}
	}
	return "", errors.New("check proxy exit ip failed")
}

func (r *Runtime) lookupIPGeo(ctx context.Context, ip string) (proxyExitGeo, error) {
	if geo, ok := r.cachedIPGeo(ip); ok {
		return geo, nil
	}
	for _, endpoint := range ipGeoLookupEndpoints(ip) {
		geo, err := requestIPInfo(ctx, http.DefaultClient, endpoint, false)
		if err == nil {
			geo.IP = ip
			r.saveIPGeoCache(ip, geo)
			return geo, nil
		}
	}
	return proxyExitGeo{}, errors.New("lookup ip geo failed")
}

func (r *Runtime) cachedIPGeo(ip string) (proxyExitGeo, bool) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return proxyExitGeo{}, false
	}
	r.geoMu.Lock()
	defer r.geoMu.Unlock()
	if r.geoCache == nil {
		return proxyExitGeo{}, false
	}
	item, ok := r.geoCache[ip]
	if !ok || time.Now().After(item.expiresAt) {
		delete(r.geoCache, ip)
		return proxyExitGeo{}, false
	}
	return item.geo, true
}

func (r *Runtime) saveIPGeoCache(ip string, geo proxyExitGeo) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	r.geoMu.Lock()
	defer r.geoMu.Unlock()
	if r.geoCache == nil {
		r.geoCache = map[string]cachedIPGeo{}
	}
	r.geoCache[ip] = cachedIPGeo{geo: geo, expiresAt: time.Now().Add(ipGeoCacheTTL)}
}

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

func normalizeConnectivityTarget(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", errors.New("target_url is required")
	}
	if !strings.Contains(value, "://") {
		value = "https://" + value
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", errors.New("target_url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("target_url scheme must be http or https")
	}
	return parsed.String(), nil
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

func enrichIPFraudGeo(check *proxyruntimev1.ProxyIPFraudCheck, geo proxyExitGeo) {
	if check == nil {
		return
	}
	if check.CountryCode == "" {
		check.CountryCode = geo.CountryCode
	}
	if check.Region == "" {
		check.Region = geo.Region
	}
	if check.City == "" {
		check.City = geo.City
	}
}

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

type edgeCanaryOutcome struct {
	level        proxyruntimev1.ProxyEdgeAccessRiskLevel
	score        float64
	signal       proxyruntimev1.ProxyEdgeAccessRiskSignal
	errorMessage string
}

func (r *Runtime) runEdgeCanary(ctx context.Context, client *http.Client, settings *runtimeSettingsFile) edgeCanaryOutcome {
	edgeCanary := settings.GetEdgeCanary()
	target := strings.TrimSpace(edgeCanary.GetUrl())
	token := strings.TrimSpace(edgeCanary.GetToken())
	if !edgeCanaryEnabled(edgeCanary) || target == "" || token == "" {
		return edgeCanaryOutcome{
			level:        proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNSUPPORTED,
			signal:       proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_ACCESS_UNSUPPORTED,
			errorMessage: "edge access canary is not configured",
		}
	}
	canaryCtx, cancel := context.WithTimeout(ctx, r.cfg.EdgeCanaryTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(canaryCtx, http.MethodGet, target, nil)
	if err != nil {
		return edgeUnavailableOutcome()
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("X-Canary-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		return edgeUnavailableOutcome()
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return edgeUnavailableOutcome()
	}
	if edgeChallengeDetected(resp.Header, body) {
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_CHALLENGE_LIKELY,
			score:  85,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_CHALLENGE_DETECTED,
		}
	}
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_HIGH,
			score:  70,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_RATE_LIMIT_DETECTED,
		}
	case resp.StatusCode == http.StatusForbidden:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_BLOCK_LIKELY,
			score:  90,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_BLOCK_DETECTED,
		}
	case resp.StatusCode >= 500 || resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnauthorized:
		return edgeUnavailableOutcome()
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return edgeCanaryOutcome{
			level:  proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_LOW,
			score:  0,
			signal: proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_ACCESS_PASSED,
		}
	default:
		return edgeUnavailableOutcome()
	}
}

func edgeUnavailableOutcome() edgeCanaryOutcome {
	return edgeCanaryOutcome{
		level:        proxyruntimev1.ProxyEdgeAccessRiskLevel_PROXY_EDGE_ACCESS_RISK_LEVEL_UNKNOWN,
		signal:       proxyruntimev1.ProxyEdgeAccessRiskSignal_PROXY_EDGE_ACCESS_RISK_SIGNAL_EDGE_UNAVAILABLE,
		errorMessage: "edge access check unavailable",
	}
}

func edgeChallengeDetected(header http.Header, body []byte) bool {
	if strings.EqualFold(strings.TrimSpace(header.Get("cf-mitigated")), "challenge") {
		return true
	}
	text := strings.ToLower(string(body))
	for _, hint := range []string{"challenge-platform", "cf-chl", "just a moment"} {
		if strings.Contains(text, hint) {
			return true
		}
	}
	return false
}

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

package mihomo

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ProviderID = "mihomo"
	groupName  = "byte-v-forge-source"
	nodeID     = "mihomo-source"
)

type Config struct {
	Path      string
	ConfigDir string
	APIAddr   string
}

type Driver struct {
	cfg    Config
	logger *slog.Logger

	mu           sync.Mutex
	cmd          *exec.Cmd
	cancel       context.CancelFunc
	configDir    string
	configPath   string
	signature    string
	running      bool
	lastError    string
	lastEndpoint sourceplane.Endpoint
}

func New(cfg Config, logger *slog.Logger) *Driver {
	if logger == nil {
		logger = slog.Default()
	}
	return &Driver{cfg: cfg, logger: logger}
}

func (d *Driver) Name() string { return ProviderID }

func (d *Driver) Reconcile(ctx context.Context, cfg sourceplane.Config) ([]provider.Node, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	file, err := d.loadSourceFileLocked(cfg.Providers)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	providers := enabledProviders(file.Subscriptions)
	fixedProxies := enabledFixedProxies(file.FixedProxies)
	if len(providers) == 0 && len(fixedProxies) == 0 {
		d.stopLocked()
		d.signature = ""
		d.lastError = "no mihomo sources configured"
		return nil, nil
	}
	endpoint, err := normalizeEndpoint(cfg.Endpoint)
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	dir, err := d.ensureConfigDir()
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	configFile, err := renderConfig(renderOptions{Providers: providers, FixedProxies: fixedProxies, Endpoint: endpoint, APIAddr: d.cfg.APIAddr, GroupStrategy: cfg.GroupStrategy, HealthCheckURL: cfg.HealthCheckURL, HealthCheckInterval: cfg.HealthCheckInterval, HealthCheckTimeout: cfg.HealthCheckTimeout})
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	data, err := json.MarshalIndent(configFile, "", "  ")
	if err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	sig := signature(data)
	if err := os.MkdirAll(filepath.Join(dir, "providers"), 0o700); err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	configPath := filepath.Join(dir, "config.json")
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		d.lastError = err.Error()
		return nil, err
	}

	restartRequired := !d.running || d.lastEndpoint != endpoint
	if restartRequired {
		d.stopLocked()
		if err := d.startLocked(ctx, dir, configPath); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
	} else if d.signature != sig {
		if err := d.reloadLocked(ctx); err != nil {
			d.lastError = err.Error()
			return nil, err
		}
	}
	if err := waitForEndpoint(ctx, endpoint.Addr, 3*time.Second); err != nil {
		d.lastError = err.Error()
		return nil, err
	}
	d.signature = sig
	d.configPath = configPath
	d.lastEndpoint = endpoint
	d.lastError = ""
	return []provider.Node{sourceNode(endpoint)}, nil
}

func (d *Driver) Sources(ctx context.Context) ([]*proxyruntimev1.ProxySourceDescriptor, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	file, err := d.loadSourceFileLocked(nil)
	if err != nil {
		return nil, err
	}
	out := make([]*proxyruntimev1.ProxySourceDescriptor, 0, len(file.Subscriptions)+len(file.FixedProxies))
	for _, item := range file.Subscriptions {
		out = append(out, subscriptionSourceDescriptor(item))
	}
	for _, item := range file.FixedProxies {
		out = append(out, fixedSourceDescriptor(item))
	}
	return out, nil
}

func (d *Driver) SourceNodes(ctx context.Context, sourceID string) ([]*proxyruntimev1.ProxySourceNode, error) {
	d.mu.Lock()
	running := d.running
	apiAddr := strings.TrimSpace(d.cfg.APIAddr)
	file, err := d.loadSourceFileLocked(nil)
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	allowed := subscriptionIDs(file.Subscriptions)
	if len(allowed) == 0 {
		return nil, nil
	}
	if id := safeID(sourceID); id != "" {
		if _, exists := allowed[id]; !exists {
			return nil, nil
		}
	}
	if !running {
		return nil, errors.New("mihomo source runtime is not running")
	}
	if apiAddr == "" {
		return nil, errors.New("mihomo api address is required")
	}
	return fetchSourceNodes(ctx, apiAddr, sourceID, allowed)
}

func (d *Driver) UpsertSubscriptionSource(ctx context.Context, req *proxyruntimev1.UpsertProxySubscriptionSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	file, err := d.loadSourceFileLocked(nil)
	if err != nil {
		return nil, err
	}
	item, providers, err := upsertProvider(file.Subscriptions, req)
	if err != nil {
		return nil, err
	}
	file.Subscriptions = providers
	if err := d.saveSourceFileLocked(file); err != nil {
		return nil, err
	}
	if !req.GetEnabled() {
		return disabledSubscriptionDescriptor(item.ID, item.DisplayName), nil
	}
	return subscriptionSourceDescriptor(item), nil
}

func (d *Driver) UpsertFixedSource(ctx context.Context, req *proxyruntimev1.UpsertProxyFixedSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	file, err := d.loadSourceFileLocked(nil)
	if err != nil {
		return nil, err
	}
	item, fixedProxies, err := upsertFixedProxy(file.FixedProxies, req)
	if err != nil {
		return nil, err
	}
	file.FixedProxies = fixedProxies
	if err := d.saveSourceFileLocked(file); err != nil {
		return nil, err
	}
	if !req.GetEnabled() {
		return disabledFixedDescriptor(item.ID, item.DisplayName), nil
	}
	return fixedSourceDescriptor(item), nil
}

func (d *Driver) DeleteSource(ctx context.Context, sourceID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	file, err := d.loadSourceFileLocked(nil)
	if err != nil {
		return err
	}
	file.Subscriptions = deleteProvider(file.Subscriptions, sourceID)
	file.FixedProxies = deleteFixedProxy(file.FixedProxies, sourceID)
	return d.saveSourceFileLocked(file)
}

func (d *Driver) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopLocked()
}

func (d *Driver) Status() sourceplane.Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return sourceplane.Status{Running: d.running, ConfigPath: d.configPath, LastError: d.lastError}
}

func (d *Driver) ensureConfigDir() (string, error) {
	if d.configDir != "" {
		return d.configDir, nil
	}
	dir := strings.TrimSpace(d.cfg.ConfigDir)
	if dir == "" {
		created, err := os.MkdirTemp("", "proxy-runtime-mihomo-")
		if err != nil {
			return "", err
		}
		dir = created
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	d.configDir = dir
	return dir, nil
}

func (d *Driver) startLocked(ctx context.Context, dir string, configPath string) error {
	path := strings.TrimSpace(d.cfg.Path)
	if path == "" {
		return errors.New("mihomo path is required")
	}
	processCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(processCtx, path, "-f", configPath)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "SAFE_PATHS="+dir)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}
	d.cmd = cmd
	d.cancel = cancel
	d.running = true
	go d.wait(cmd)
	return nil
}

func (d *Driver) reloadLocked(ctx context.Context) error {
	if strings.TrimSpace(d.cfg.APIAddr) == "" {
		return errors.New("mihomo api address is required for hot reload")
	}
	body := []byte(`{"path":"config.json"}`)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, controlURL(d.cfg.APIAddr, "/configs?force=true"), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("mihomo config reload returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
}

type mihomoProvidersResponse struct {
	Providers map[string]mihomoProxyProviderState `json:"providers"`
}

type mihomoProxyProviderState struct {
	Name    string             `json:"name"`
	Proxies []mihomoProxyState `json:"proxies"`
}

type mihomoProxyState struct {
	Name    string               `json:"name"`
	Type    string               `json:"type"`
	Alive   *bool                `json:"alive"`
	History []mihomoProxyHistory `json:"history"`
}

type mihomoProxyHistory struct {
	Time    string `json:"time"`
	Delay   int64  `json:"delay"`
	Message string `json:"message"`
}

func fetchSourceNodes(ctx context.Context, apiAddr string, sourceID string, allowed map[string]struct{}) ([]*proxyruntimev1.ProxySourceNode, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, controlURL(apiAddr, "/providers/proxies"), nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("mihomo providers returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var payload mihomoProvidersResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&payload); err != nil {
		return nil, err
	}
	filterID := safeID(sourceID)
	nodes := make([]*proxyruntimev1.ProxySourceNode, 0)
	for providerID, providerState := range payload.Providers {
		id := safeID(firstNonEmpty(providerState.Name, providerID))
		if _, exists := allowed[id]; !exists {
			continue
		}
		if filterID != "" && id != filterID {
			continue
		}
		for _, proxyState := range providerState.Proxies {
			nodes = append(nodes, sourceNodeFromMihomo(id, proxyState))
		}
	}
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].GetSourceId() == nodes[j].GetSourceId() {
			return nodes[i].GetDisplayName() < nodes[j].GetDisplayName()
		}
		return nodes[i].GetSourceId() < nodes[j].GetSourceId()
	})
	return nodes, nil
}

func subscriptionIDs(subscriptions []sourceplane.SubscriptionProvider) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range subscriptions {
		if id := safeID(item.ID); id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func sourceNodeFromMihomo(sourceID string, state mihomoProxyState) *proxyruntimev1.ProxySourceNode {
	history := latestHistory(state.History)
	name := firstNonEmpty(state.Name, "unnamed")
	status := proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_UNKNOWN
	if state.Alive != nil {
		if *state.Alive {
			status = proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_AVAILABLE
		} else {
			status = proxyruntimev1.ProxySourceNodeStatus_PROXY_SOURCE_NODE_STATUS_UNAVAILABLE
		}
	}
	return &proxyruntimev1.ProxySourceNode{
		SourceId:     sourceID,
		NodeId:       sourceID + "/" + safeID(name),
		DisplayName:  name,
		NodeType:     strings.ToLower(strings.TrimSpace(state.Type)),
		Status:       status,
		DelayMs:      delayMillis(history.Delay),
		CheckedAt:    historyTimestamp(history.Time),
		ErrorMessage: strings.TrimSpace(history.Message),
	}
}

func latestHistory(history []mihomoProxyHistory) mihomoProxyHistory {
	if len(history) == 0 {
		return mihomoProxyHistory{}
	}
	return history[len(history)-1]
}

func delayMillis(value int64) uint32 {
	if value <= 0 {
		return 0
	}
	if value > int64(^uint32(0)) {
		return ^uint32(0)
	}
	return uint32(value)
}

func historyTimestamp(value string) *timestamppb.Timestamp {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil
	}
	return timestamppb.New(parsed)
}

func controlURL(apiAddr string, path string) string {
	apiAddr = strings.TrimRight(strings.TrimSpace(apiAddr), "/")
	if strings.HasPrefix(apiAddr, "http://") || strings.HasPrefix(apiAddr, "https://") {
		return apiAddr + path
	}
	return "http://" + apiAddr + path
}

func (d *Driver) wait(cmd *exec.Cmd) {
	err := cmd.Wait()
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.cmd != cmd {
		return
	}
	d.running = false
	if err != nil {
		d.lastError = err.Error()
	}
}

func (d *Driver) stopLocked() {
	cancel := d.cancel
	cmd := d.cmd
	d.cancel = nil
	d.cmd = nil
	d.running = false
	if cancel != nil {
		cancel()
	}
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
}

type sourceFile struct {
	Subscriptions []sourceplane.SubscriptionProvider `json:"subscriptions"`
	FixedProxies  []sourceplane.FixedProxy           `json:"fixed_proxies"`
}

func (d *Driver) loadSourceFileLocked(bootstrap []sourceplane.SubscriptionProvider) (sourceFile, error) {
	dir, err := d.ensureConfigDir()
	if err != nil {
		return sourceFile{}, err
	}
	path := sourcesPath(dir)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		file := cleanSourceFile(sourceFile{Subscriptions: bootstrap})
		if len(file.Subscriptions) > 0 || len(file.FixedProxies) > 0 {
			return file, d.saveSourceFileLocked(file)
		}
		return sourceFile{}, nil
	}
	if err != nil {
		return sourceFile{}, err
	}
	var file sourceFile
	if len(data) > 0 {
		if err := json.Unmarshal(data, &file); err != nil {
			return sourceFile{}, err
		}
	}
	file = cleanSourceFile(file)
	if len(file.Subscriptions) == 0 && len(file.FixedProxies) == 0 && len(bootstrap) > 0 {
		file = cleanSourceFile(sourceFile{Subscriptions: bootstrap})
		return file, d.saveSourceFileLocked(file)
	}
	return file, nil
}

func (d *Driver) saveSourceFileLocked(file sourceFile) error {
	dir, err := d.ensureConfigDir()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cleanSourceFile(file), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sourcesPath(dir), data, 0o600)
}

func sourcesPath(dir string) string { return filepath.Join(dir, "sources.json") }

func cleanSourceFile(file sourceFile) sourceFile {
	return sourceFile{Subscriptions: cleanProviders(file.Subscriptions), FixedProxies: cleanFixedProxies(file.FixedProxies)}
}

func cleanProviders(in []sourceplane.SubscriptionProvider) []sourceplane.SubscriptionProvider {
	seen := map[string]struct{}{}
	out := make([]sourceplane.SubscriptionProvider, 0, len(in))
	for _, item := range in {
		item.ID = safeID(item.ID)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.URL = strings.TrimSpace(item.URL)
		item.Path = strings.TrimSpace(item.Path)
		item.Filter = strings.TrimSpace(item.Filter)
		item.ExcludeFilter = strings.TrimSpace(item.ExcludeFilter)
		item.HealthCheckURL = strings.TrimSpace(item.HealthCheckURL)
		item.RegionCodes = cleanRegionCodes(item.RegionCodes)
		if item.ID == "" || item.URL == "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cleanFixedProxies(in []sourceplane.FixedProxy) []sourceplane.FixedProxy {
	seen := map[string]struct{}{}
	out := make([]sourceplane.FixedProxy, 0, len(in))
	for _, item := range in {
		item.ID = safeID(item.ID)
		item.DisplayName = strings.TrimSpace(item.DisplayName)
		item.URI = strings.TrimSpace(item.URI)
		item.RegionCodes = cleanRegionCodes(item.RegionCodes)
		if item.ID == "" || item.URI == "" {
			continue
		}
		if _, exists := seen[item.ID]; exists {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}

func cleanRegionCodes(in []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(in))
	for _, value := range in {
		value = strings.ToUpper(strings.TrimSpace(value))
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func upsertProvider(providers []sourceplane.SubscriptionProvider, req *proxyruntimev1.UpsertProxySubscriptionSourceRequest) (sourceplane.SubscriptionProvider, []sourceplane.SubscriptionProvider, error) {
	id, err := requestID(req.GetSourceId(), "sub", providerIDs(providers))
	if err != nil {
		return sourceplane.SubscriptionProvider{}, nil, err
	}
	current := sourceplane.SubscriptionProvider{ID: id}
	found := false
	for _, item := range providers {
		if item.ID == id {
			current = item
			found = true
			break
		}
	}
	current.DisplayName = firstNonEmpty(req.GetDisplayName(), current.DisplayName, "Subscription")
	if !req.GetEnabled() {
		return current, deleteProvider(providers, id), nil
	}
	if req.GetClearUrl() {
		current.URL = ""
	}
	if strings.TrimSpace(req.GetUrl()) != "" {
		current.URL = strings.TrimSpace(req.GetUrl())
	}
	if current.URL == "" {
		return sourceplane.SubscriptionProvider{}, nil, errors.New("enabled subscription requires url")
	}
	current.Interval = protoDuration(req.GetInterval(), defaultDuration(current.Interval, time.Hour))
	current.Filter = strings.TrimSpace(req.GetFilter())
	current.ExcludeFilter = strings.TrimSpace(req.GetExcludeFilter())
	current.HealthCheckURL = strings.TrimSpace(req.GetHealthCheckUrl())
	current.HealthInterval = protoDuration(req.GetHealthInterval(), defaultDuration(current.HealthInterval, 300*time.Second))
	current.HealthTimeout = protoDuration(req.GetHealthTimeout(), defaultDuration(current.HealthTimeout, 5*time.Second))
	current.HealthLazy = req.GetHealthLazy()
	current.ExpectedStatus = req.GetExpectedStatus()
	current.RegionCodes = cleanRegionCodes(req.GetRegionCodes())
	if current.ExpectedStatus == 0 {
		current.ExpectedStatus = 204
	}
	return current, replaceProvider(providers, current, found), nil
}

func upsertFixedProxy(fixedProxies []sourceplane.FixedProxy, req *proxyruntimev1.UpsertProxyFixedSourceRequest) (sourceplane.FixedProxy, []sourceplane.FixedProxy, error) {
	id, err := requestID(req.GetSourceId(), "fixed", fixedIDs(fixedProxies))
	if err != nil {
		return sourceplane.FixedProxy{}, nil, err
	}
	current := sourceplane.FixedProxy{ID: id}
	found := false
	for _, item := range fixedProxies {
		if item.ID == id {
			current = item
			found = true
			break
		}
	}
	current.DisplayName = firstNonEmpty(req.GetDisplayName(), current.DisplayName, fixedName(req.GetUri()), "Fixed proxy")
	if !req.GetEnabled() {
		return current, deleteFixedProxy(fixedProxies, id), nil
	}
	if req.GetClearUri() {
		current.URI = ""
	}
	if strings.TrimSpace(req.GetUri()) != "" {
		current.URI = strings.TrimSpace(req.GetUri())
	}
	if current.URI == "" {
		return sourceplane.FixedProxy{}, nil, errors.New("enabled fixed proxy requires uri")
	}
	if _, err := renderFixedProxy(current); err != nil {
		return sourceplane.FixedProxy{}, nil, err
	}
	current.RegionCodes = cleanRegionCodes(req.GetRegionCodes())
	return current, replaceFixedProxy(fixedProxies, current, found), nil
}

func replaceProvider(providers []sourceplane.SubscriptionProvider, current sourceplane.SubscriptionProvider, found bool) []sourceplane.SubscriptionProvider {
	out := make([]sourceplane.SubscriptionProvider, 0, len(providers)+1)
	if !found {
		out = append(out, current)
	} else {
		for _, item := range providers {
			if item.ID == current.ID {
				out = append(out, current)
				continue
			}
			out = append(out, item)
		}
	}
	return cleanProviders(out)
}

func replaceFixedProxy(fixedProxies []sourceplane.FixedProxy, current sourceplane.FixedProxy, found bool) []sourceplane.FixedProxy {
	out := make([]sourceplane.FixedProxy, 0, len(fixedProxies)+1)
	if !found {
		out = append(out, current)
	} else {
		for _, item := range fixedProxies {
			if item.ID == current.ID {
				out = append(out, current)
				continue
			}
			out = append(out, item)
		}
	}
	return cleanFixedProxies(out)
}

func deleteProvider(providers []sourceplane.SubscriptionProvider, sourceID string) []sourceplane.SubscriptionProvider {
	id := safeID(sourceID)
	out := make([]sourceplane.SubscriptionProvider, 0, len(providers))
	for _, item := range providers {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return cleanProviders(out)
}

func deleteFixedProxy(fixedProxies []sourceplane.FixedProxy, sourceID string) []sourceplane.FixedProxy {
	id := safeID(sourceID)
	out := make([]sourceplane.FixedProxy, 0, len(fixedProxies))
	for _, item := range fixedProxies {
		if item.ID != id {
			out = append(out, item)
		}
	}
	return cleanFixedProxies(out)
}

func subscriptionSourceDescriptor(item sourceplane.SubscriptionProvider) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: item.ID, ProviderId: ProviderID, DisplayName: firstNonEmpty(item.DisplayName, item.ID), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_SUBSCRIPTION_PROVIDER, proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_POOL_REFRESH}, Model: &proxyruntimev1.ProxySourceDescriptor_Subscription{Subscription: &proxyruntimev1.ProxySubscriptionSourceDescriptor{UrlRedacted: redactedURL(item.URL), Interval: durationpb.New(defaultDuration(item.Interval, time.Hour)), Filter: item.Filter, ExcludeFilter: item.ExcludeFilter, HealthCheckUrl: item.HealthCheckURL, HealthInterval: durationpb.New(defaultDuration(item.HealthInterval, 300*time.Second)), HealthTimeout: durationpb.New(defaultDuration(item.HealthTimeout, 5*time.Second)), HealthLazy: item.HealthLazy, ExpectedStatus: defaultExpectedStatus(item.ExpectedStatus), RegionCodes: cleanRegionCodes(item.RegionCodes)}}}
}

func disabledSubscriptionDescriptor(id string, displayName string) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: id, ProviderId: ProviderID, DisplayName: firstNonEmpty(displayName, id), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_SUBSCRIPTION, Enabled: false, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_SUBSCRIPTION_PROVIDER, proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_POOL_REFRESH}, Model: &proxyruntimev1.ProxySourceDescriptor_Subscription{Subscription: &proxyruntimev1.ProxySubscriptionSourceDescriptor{}}}
}

func fixedSourceDescriptor(item sourceplane.FixedProxy) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: item.ID, ProviderId: ProviderID, DisplayName: firstNonEmpty(item.DisplayName, item.ID), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: true, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{EndpointCount: 1, RegionCodes: cleanRegionCodes(item.RegionCodes)}}}
}

func disabledFixedDescriptor(id string, displayName string) *proxyruntimev1.ProxySourceDescriptor {
	return &proxyruntimev1.ProxySourceDescriptor{SourceId: id, ProviderId: ProviderID, DisplayName: firstNonEmpty(displayName, id), Kind: proxyruntimev1.ProxySourceKind_PROXY_SOURCE_KIND_FIXED_PROXY, Enabled: false, Capabilities: []proxyruntimev1.ProxyCapability{proxyruntimev1.ProxyCapability_PROXY_CAPABILITY_CHAINING}, Model: &proxyruntimev1.ProxySourceDescriptor_FixedProxy{FixedProxy: &proxyruntimev1.ProxyFixedSourceDescriptor{}}}
}

func redactedURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	return "configured"
}

func protoDuration(value *durationpb.Duration, fallback time.Duration) time.Duration {
	if value == nil || value.AsDuration() <= 0 {
		return fallback
	}
	return value.AsDuration()
}

func defaultDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value <= 0 {
		return fallback
	}
	return value
}

type renderOptions struct {
	Providers           []sourceplane.SubscriptionProvider
	FixedProxies        []sourceplane.FixedProxy
	Endpoint            sourceplane.Endpoint
	APIAddr             string
	GroupStrategy       string
	HealthCheckURL      string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
}

type mihomoConfig struct {
	MixedPort          int                       `json:"mixed-port"`
	BindAddress        string                    `json:"bind-address,omitempty"`
	AllowLAN           bool                      `json:"allow-lan"`
	Mode               string                    `json:"mode"`
	LogLevel           string                    `json:"log-level"`
	ExternalController string                    `json:"external-controller,omitempty"`
	Proxies            []map[string]any          `json:"proxies,omitempty"`
	ProxyProviders     map[string]mihomoProvider `json:"proxy-providers,omitempty"`
	ProxyGroups        []mihomoGroup             `json:"proxy-groups,omitempty"`
	Rules              []string                  `json:"rules"`
}

type mihomoProvider struct {
	Type        string              `json:"type"`
	URL         string              `json:"url"`
	Path        string              `json:"path,omitempty"`
	Interval    int                 `json:"interval,omitempty"`
	Filter      string              `json:"filter,omitempty"`
	Exclude     string              `json:"exclude-filter,omitempty"`
	HealthCheck *mihomoHealthCheck  `json:"health-check,omitempty"`
	Header      map[string][]string `json:"header,omitempty"`
}

type mihomoHealthCheck struct {
	Enable         bool   `json:"enable"`
	URL            string `json:"url,omitempty"`
	Interval       int    `json:"interval,omitempty"`
	Timeout        int    `json:"timeout,omitempty"`
	Lazy           bool   `json:"lazy"`
	ExpectedStatus uint32 `json:"expected-status,omitempty"`
}

type mihomoGroup struct {
	Name           string   `json:"name"`
	Type           string   `json:"type"`
	Proxies        []string `json:"proxies,omitempty"`
	Use            []string `json:"use,omitempty"`
	URL            string   `json:"url,omitempty"`
	Interval       int      `json:"interval,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	Lazy           bool     `json:"lazy"`
	ExpectedStatus uint32   `json:"expected-status,omitempty"`
}

func renderConfig(opts renderOptions) (mihomoConfig, error) {
	host, port, err := splitEndpoint(opts.Endpoint.Addr)
	if err != nil {
		return mihomoConfig{}, err
	}
	providerMap := make(map[string]mihomoProvider, len(opts.Providers))
	providerIDs := make([]string, 0, len(opts.Providers))
	for _, item := range opts.Providers {
		id := safeID(item.ID)
		providerIDs = append(providerIDs, id)
		providerMap[id] = mihomoProvider{
			Type:     "http",
			URL:      item.URL,
			Path:     providerPath(item, id),
			Interval: seconds(item.Interval, 3600),
			Filter:   item.Filter,
			Exclude:  item.ExcludeFilter,
			Header:   item.Headers,
			HealthCheck: &mihomoHealthCheck{
				Enable:         true,
				URL:            firstNonEmpty(item.HealthCheckURL, opts.HealthCheckURL, "https://www.gstatic.com/generate_204"),
				Interval:       seconds(item.HealthInterval, secondsDuration(opts.HealthCheckInterval, 300)),
				Timeout:        milliseconds(item.HealthTimeout, millisecondsDuration(opts.HealthCheckTimeout, 5000)),
				Lazy:           item.HealthLazy,
				ExpectedStatus: defaultExpectedStatus(item.ExpectedStatus),
			},
		}
	}
	fixedConfigs := make([]map[string]any, 0, len(opts.FixedProxies))
	fixedIDs := make([]string, 0, len(opts.FixedProxies))
	for _, item := range opts.FixedProxies {
		proxyConfig, err := renderFixedProxy(item)
		if err != nil {
			return mihomoConfig{}, err
		}
		fixedConfigs = append(fixedConfigs, proxyConfig)
		fixedIDs = append(fixedIDs, safeID(item.ID))
	}
	return mihomoConfig{
		MixedPort:          port,
		BindAddress:        host,
		AllowLAN:           false,
		Mode:               "rule",
		LogLevel:           "warning",
		ExternalController: strings.TrimSpace(opts.APIAddr),
		Proxies:            fixedConfigs,
		ProxyProviders:     providerMap,
		ProxyGroups: []mihomoGroup{{
			Name:           groupName,
			Type:           groupStrategy(opts.GroupStrategy),
			Proxies:        fixedIDs,
			Use:            providerIDs,
			URL:            firstNonEmpty(opts.HealthCheckURL, "https://www.gstatic.com/generate_204"),
			Interval:       secondsDuration(opts.HealthCheckInterval, 300),
			Timeout:        millisecondsDuration(opts.HealthCheckTimeout, 5000),
			Lazy:           true,
			ExpectedStatus: 204,
		}},
		Rules: []string{"MATCH," + groupName},
	}, nil
}

func renderFixedProxy(item sourceplane.FixedProxy) (map[string]any, error) {
	parsed, err := url.Parse(strings.TrimSpace(item.URI))
	if err != nil {
		return nil, fmt.Errorf("parse fixed proxy uri: %w", err)
	}
	if strings.ToLower(parsed.Scheme) != "vless" {
		return nil, fmt.Errorf("unsupported fixed proxy scheme %q", parsed.Scheme)
	}
	host := parsed.Hostname()
	portValue := parsed.Port()
	if host == "" || portValue == "" || parsed.User == nil {
		return nil, errors.New("vless uri requires uuid, host and port")
	}
	var port int
	if _, err := fmt.Sscanf(portValue, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return nil, fmt.Errorf("invalid vless port %q", portValue)
	}
	name := safeID(item.ID)
	if name == "" {
		return nil, errors.New("fixed proxy id is required")
	}
	query := parsed.Query()
	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	network := strings.ToLower(firstNonEmpty(query.Get("type"), query.Get("network"), "tcp"))
	config := map[string]any{
		"name":       name,
		"type":       "vless",
		"server":     host,
		"port":       port,
		"uuid":       parsed.User.Username(),
		"udp":        true,
		"network":    network,
		"encryption": firstNonEmpty(query.Get("encryption"), "none"),
	}
	if flow := strings.TrimSpace(query.Get("flow")); flow != "" {
		config["flow"] = flow
	}
	if fingerprint := firstNonEmpty(query.Get("fp"), query.Get("client-fingerprint")); fingerprint != "" {
		config["client-fingerprint"] = fingerprint
	}
	if security == "tls" || security == "reality" {
		config["tls"] = true
	}
	if serverName := firstNonEmpty(query.Get("sni"), query.Get("servername")); serverName != "" {
		config["servername"] = serverName
	}
	if security == "reality" {
		reality := map[string]any{}
		if value := firstNonEmpty(query.Get("pbk"), query.Get("public-key")); value != "" {
			reality["public-key"] = value
		}
		if value := firstNonEmpty(query.Get("sid"), query.Get("short-id")); value != "" {
			reality["short-id"] = value
		}
		if len(reality) > 0 {
			config["reality-opts"] = reality
		}
	}
	switch network {
	case "ws", "websocket":
		config["network"] = "ws"
		opts := map[string]any{}
		if value := strings.TrimSpace(query.Get("path")); value != "" {
			opts["path"] = value
		}
		if value := strings.TrimSpace(query.Get("host")); value != "" {
			opts["headers"] = map[string]string{"Host": value}
		}
		if len(opts) > 0 {
			config["ws-opts"] = opts
		}
	case "grpc":
		opts := map[string]any{}
		if value := firstNonEmpty(query.Get("serviceName"), query.Get("service-name")); value != "" {
			opts["grpc-service-name"] = value
		}
		if len(opts) > 0 {
			config["grpc-opts"] = opts
		}
	case "tcp":
	default:
		return nil, fmt.Errorf("unsupported vless network %q", network)
	}
	return config, nil
}

func sourceNode(endpoint sourceplane.Endpoint) provider.Node {
	proxyURL := &url.URL{Scheme: "socks5", Host: endpoint.Addr}
	return provider.Node{
		ID:           nodeID,
		URL:          proxyURL,
		ProviderID:   ProviderID,
		UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_PROXY_POOL,
		RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_SCHEDULED_POOL_REFRESH,
		Labels: map[string]string{
			"mode":           "subscription_source_plane",
			"source_runtime": ProviderID,
		},
	}
}

func enabledProviders(in []sourceplane.SubscriptionProvider) []sourceplane.SubscriptionProvider {
	out := make([]sourceplane.SubscriptionProvider, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URL) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func enabledFixedProxies(in []sourceplane.FixedProxy) []sourceplane.FixedProxy {
	out := make([]sourceplane.FixedProxy, 0, len(in))
	for _, item := range in {
		if strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.URI) == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func normalizeEndpoint(endpoint sourceplane.Endpoint) (sourceplane.Endpoint, error) {
	endpoint.Addr = strings.TrimSpace(endpoint.Addr)
	if endpoint.Addr == "" {
		endpoint.Addr = "127.0.0.1:18900"
	}
	if _, _, err := splitEndpoint(endpoint.Addr); err != nil {
		return sourceplane.Endpoint{}, err
	}
	endpoint.Protocol = strings.ToLower(strings.TrimSpace(endpoint.Protocol))
	if endpoint.Protocol == "" {
		endpoint.Protocol = "socks5"
	}
	if endpoint.Protocol != "socks5" {
		return sourceplane.Endpoint{}, fmt.Errorf("unsupported mihomo source endpoint protocol %q", endpoint.Protocol)
	}
	return endpoint, nil
}

func splitEndpoint(addr string) (string, int, error) {
	host, portValue, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	var port int
	if _, err := fmt.Sscanf(portValue, "%d", &port); err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("invalid mihomo mixed port %q", portValue)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	return host, port, nil
}

func waitForEndpoint(ctx context.Context, addr string, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		conn, err := (&net.Dialer{Timeout: 100 * time.Millisecond}).DialContext(waitCtx, "tcp", addr)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("mihomo mixed listener %s is not ready: %w", addr, waitCtx.Err())
		case <-ticker.C:
		}
	}
}

func signature(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func safeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var out strings.Builder
	for _, r := range value {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' {
			out.WriteRune(r)
			continue
		}
		out.WriteByte('-')
	}
	return strings.Trim(out.String(), "-")
}

func requestID(input string, prefix string, existing map[string]struct{}) (string, error) {
	id := safeID(input)
	if id != "" {
		return id, nil
	}
	for range 8 {
		suffix, err := randx.Hex(6)
		if err != nil {
			return "", err
		}
		id = prefix + "-" + suffix
		if _, exists := existing[id]; !exists {
			return id, nil
		}
	}
	return "", errors.New("generate source id failed")
}

func providerIDs(providers []sourceplane.SubscriptionProvider) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range providers {
		if id := safeID(item.ID); id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func fixedIDs(fixedProxies []sourceplane.FixedProxy) map[string]struct{} {
	out := map[string]struct{}{}
	for _, item := range fixedProxies {
		if id := safeID(item.ID); id != "" {
			out[id] = struct{}{}
		}
	}
	return out
}

func fixedName(rawURI string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURI))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Fragment)
}

func providerPath(item sourceplane.SubscriptionProvider, id string) string {
	if strings.TrimSpace(item.Path) != "" {
		return item.Path
	}
	return filepath.Join("providers", id+".yaml")
}

func groupStrategy(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "select", "url-test", "load-balance":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "fallback"
	}
}

func seconds(value time.Duration, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return int(value / time.Second)
}

func milliseconds(value time.Duration, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return int(value / time.Millisecond)
}

func secondsDuration(value time.Duration, fallback int) int { return seconds(value, fallback) }
func millisecondsDuration(value time.Duration, fallback int) int {
	return milliseconds(value, fallback)
}

func defaultExpectedStatus(value uint32) uint32 {
	if value == 0 {
		return 204
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

package mihomo

import (
	"context"
	"errors"
	"strings"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

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

func (d *Driver) ResolveNodePublicIP(ctx context.Context, sourceID string, nodeID string, nodeDisplayName string) (string, error) {
	d.mu.Lock()
	file, err := d.loadSourceFileLocked(nil)
	if err != nil {
		d.mu.Unlock()
		return "", err
	}
	fixed := fixedProxyByID(file.FixedProxies, sourceID)
	if fixed != nil {
		uri := fixed.URI
		d.mu.Unlock()
		return publicIP(ctx, fixedProxyHost(uri))
	}
	providerPaths := d.providerFileCandidatesLocked(file.Subscriptions, sourceID)
	d.mu.Unlock()
	host := providerNodeHost(providerPaths, nodeID, nodeDisplayName)
	return publicIP(ctx, host)
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

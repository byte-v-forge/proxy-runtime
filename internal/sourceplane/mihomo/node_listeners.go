package mihomo

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func (d *Driver) nodeListenersLocked(ctx context.Context, endpoint sourceplane.Endpoint, providers []sourceplane.SubscriptionProvider, fixedProxies []sourceplane.FixedProxy) ([]nodeListener, error) {
	host, basePort, err := splitEndpoint(endpoint.Addr)
	if err != nil {
		return nil, err
	}
	bindings := make([]nodeListener, 0, len(fixedProxies))
	for _, item := range fixedProxies {
		sourceID := safeID(item.ID)
		if sourceID == "" {
			continue
		}
		bindings = append(bindings, nodeListener{
			SourceID:       sourceID,
			NodeID:         sourceID,
			DisplayName:    firstNonEmpty(item.DisplayName, sourceID),
			ProxyName:      sourceID,
			ProviderBacked: false,
		})
	}
	if len(providers) > 0 {
		apiAddr := strings.TrimSpace(d.cfg.APIAddr)
		if apiAddr == "" {
			return nil, errors.New("mihomo api address is required")
		}
		nodes, err := fetchSourceNodes(ctx, apiAddr, "", subscriptionIDs(providers))
		if err != nil {
			return nil, err
		}
		for _, node := range nodes {
			proxyName := strings.TrimSpace(node.GetDisplayName())
			if proxyName == "" {
				continue
			}
			bindings = append(bindings, nodeListener{
				SourceID:       node.GetSourceId(),
				NodeID:         node.GetNodeId(),
				DisplayName:    proxyName,
				ProxyName:      proxyName,
				ProviderBacked: true,
			})
		}
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].SourceID == bindings[j].SourceID {
			return bindings[i].NodeID < bindings[j].NodeID
		}
		return bindings[i].SourceID < bindings[j].SourceID
	})
	if err := assignNodeListenerPorts(bindings, host, basePort); err != nil {
		return nil, err
	}
	return bindings, nil
}

func assignNodeListenerPorts(bindings []nodeListener, host string, basePort int) error {
	if len(bindings) > nodeListenerPortSpan {
		return fmt.Errorf("mihomo node listeners exceed reserved port span: nodes=%d span=%d", len(bindings), nodeListenerPortSpan)
	}
	used := make(map[int]struct{}, len(bindings))
	for index := range bindings {
		start := int(hashPortSlot(bindings[index].SourceID+"/"+bindings[index].NodeID, uint32(nodeListenerPortSpan)))
		port := 0
		for offset := 0; offset < nodeListenerPortSpan; offset++ {
			candidate := basePort + nodeListenerPortStart + ((start + offset) % nodeListenerPortSpan)
			if candidate <= 0 || candidate > 65535 {
				return fmt.Errorf("mihomo node listener port overflow: base=%d port=%d", basePort, candidate)
			}
			if _, exists := used[candidate]; exists {
				continue
			}
			used[candidate] = struct{}{}
			port = candidate
			break
		}
		if port == 0 {
			return fmt.Errorf("mihomo node listener port allocation failed: nodes=%d span=%d", len(bindings), nodeListenerPortSpan)
		}
		bindings[index].Endpoint = sourceplane.Endpoint{Addr: net.JoinHostPort(host, fmt.Sprintf("%d", port)), Protocol: "socks5"}
	}
	return nil
}

func hashPortSlot(value string, modulo uint32) uint32 {
	if modulo == 0 {
		return 0
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(value)))
	return binary.BigEndian.Uint32(sum[:4]) % modulo
}

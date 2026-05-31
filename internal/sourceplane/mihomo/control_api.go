package mihomo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"google.golang.org/protobuf/types/known/timestamppb"
)

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
		NodeId:       sourceNodeID(sourceID, name),
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

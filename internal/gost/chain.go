package gost

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

func buildChain(name string, staticChain []*url.URL, pool []provider.Node) (Chain, error) {
	chain := Chain{Name: name}
	for index, upstream := range staticChain {
		node, err := nodeFromURL(fmt.Sprintf("static-%d", index), upstream)
		if err != nil {
			return Chain{}, err
		}
		chain.Hops = append(chain.Hops, Hop{
			Name:  fmt.Sprintf("static-hop-%d", index),
			Nodes: []Node{node},
		})
	}
	if len(pool) > 0 {
		nodes := make([]Node, 0, len(pool))
		for index, item := range pool {
			node, err := nodeFromURL(fmt.Sprintf("pool-%d", index), item.URL)
			if err != nil {
				return Chain{}, err
			}
			nodes = append(nodes, node)
		}
		chain.Hops = append(chain.Hops, Hop{Name: "provider-pool", Nodes: nodes})
	}
	return chain, nil
}

func buildUpstreamChain(listener LocalService) (Chain, error) {
	proxyURL, err := proxyurl.Parse(listener.Upstream, "http")
	if err != nil {
		return Chain{}, fmt.Errorf("parse listener %q upstream: %w", listener.Name, err)
	}
	name := safeChainName(listener.Name)
	node, err := nodeFromURL("upstream-0", proxyURL)
	if err != nil {
		return Chain{}, err
	}
	return Chain{
		Name: name,
		Hops: []Hop{{
			Name:  "upstream-hop-0",
			Nodes: []Node{node},
		}},
	}, nil
}

func safeChainName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "listener-upstream-chain"
	}
	var out strings.Builder
	for _, r := range strings.ToLower(value) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			out.WriteRune(r)
			continue
		}
		out.WriteByte('-')
	}
	name := strings.Trim(out.String(), "-")
	if name == "" {
		name = "listener-upstream"
	}
	if strings.HasSuffix(name, "-chain") {
		return name
	}
	return name + "-chain"
}

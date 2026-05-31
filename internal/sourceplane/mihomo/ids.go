package mihomo

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
)

func signature(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func lineListenerName(sourceID string, nodeID string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(sourceID) + "/" + strings.TrimSpace(nodeID)))
	return "bvf-line-" + hex.EncodeToString(sum[:])[:16]
}

func lineGroupName(sourceID string, nodeID string) string {
	sum := sha256.Sum256([]byte("group/" + strings.TrimSpace(sourceID) + "/" + strings.TrimSpace(nodeID)))
	return "bvf-line-group-" + hex.EncodeToString(sum[:])[:16]
}

func sourceNodeID(sourceID string, name string) string {
	return strings.TrimSpace(sourceID) + "/" + sourceNodeKey(name)
}

func sourceNodeKey(name string) string {
	slug := safeID(name)
	if slug == "" {
		slug = "node"
	}
	if len(slug) > 48 {
		slug = strings.Trim(slug[:48], "-")
		if slug == "" {
			slug = "node"
		}
	}
	sum := sha256.Sum256([]byte(strings.TrimSpace(name)))
	return slug + "-" + hex.EncodeToString(sum[:])[:8]
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

func providerConfigPath(configDir string, item sourceplane.SubscriptionProvider, id string) string {
	path := providerPath(item, id)
	if filepath.IsAbs(path) {
		return path
	}
	if strings.TrimSpace(configDir) == "" {
		return path
	}
	return filepath.Join(configDir, path)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

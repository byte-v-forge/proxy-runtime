package mihomo

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/byte-v-forge/proxy-runtime/internal/sourceplane"
	"gopkg.in/yaml.v3"
)

type sourceFile struct {
	Subscriptions []sourceplane.SubscriptionProvider `json:"subscriptions"`
	FixedProxies  []sourceplane.FixedProxy           `json:"fixed_proxies"`
}

type providerProxyFile struct {
	Proxies []providerProxyNode `yaml:"proxies"`
}

type providerProxyNode struct {
	Name   string `yaml:"name"`
	Server string `yaml:"server"`
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

func (d *Driver) providerFileCandidatesLocked(providers []sourceplane.SubscriptionProvider, sourceID string) []string {
	dir, err := d.ensureConfigDir()
	if err != nil {
		return nil
	}
	for _, item := range providers {
		id := safeID(item.ID)
		if id != "" && id == safeID(sourceID) {
			return providerFileCandidates(dir, item, id)
		}
	}
	return nil
}

func providerNodeHost(paths []string, nodeID string, nodeDisplayName string) string {
	targetID := strings.TrimSpace(filepath.Base(nodeID))
	targetName := strings.TrimSpace(nodeDisplayName)
	fallback := ""
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil || len(data) == 0 {
			continue
		}
		var file providerProxyFile
		if yaml.Unmarshal(data, &file) != nil {
			continue
		}
		for _, node := range file.Proxies {
			name := strings.TrimSpace(node.Name)
			host := strings.TrimSpace(node.Server)
			if name == "" || host == "" {
				continue
			}
			if targetName != "" && name == targetName {
				return host
			}
			if fallback == "" && sourceNodeKey(name) == targetID {
				fallback = host
			}
		}
	}
	return fallback
}

func providerFileCandidates(configDir string, item sourceplane.SubscriptionProvider, id string) []string {
	path := providerPath(item, id)
	out := make([]string, 0, 3)
	if filepath.IsAbs(path) {
		out = append(out, path)
	} else {
		out = append(out, filepath.Join(configDir, path))
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			out = append(out, filepath.Join(home, ".config", "mihomo", path))
		}
	}
	return out
}

func fixedProxyByID(items []sourceplane.FixedProxy, sourceID string) *sourceplane.FixedProxy {
	sourceID = safeID(sourceID)
	for i := range items {
		if safeID(items[i].ID) == sourceID {
			return &items[i]
		}
	}
	return nil
}

func fixedProxyHost(rawURI string) string {
	parsed, err := url.Parse(strings.TrimSpace(rawURI))
	if err != nil || parsed == nil {
		return ""
	}
	return parsed.Hostname()
}

func publicIP(ctx context.Context, host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String(), nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	addrs, err := net.DefaultResolver.LookupIPAddr(lookupCtx, host)
	if err != nil {
		return "", err
	}
	for _, addr := range addrs {
		if ip := addr.IP.To4(); ip != nil {
			return ip.String(), nil
		}
	}
	if len(addrs) > 0 {
		return addrs[0].IP.String(), nil
	}
	return "", nil
}

package gost

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

func nodeFromURL(name string, proxyURL *url.URL) (Node, error) {
	if proxyURL == nil || proxyURL.Host == "" {
		return Node{}, errors.New("proxy node host is required")
	}
	connector, dialer := splitNodeScheme(proxyURL.Scheme)
	node := Node{
		Name:      name,
		Addr:      proxyURL.Host,
		Connector: Connector{Type: connector},
		Dialer:    Dialer{Type: dialer},
	}
	if proxyURL.User != nil {
		password, _ := proxyURL.User.Password()
		node.Connector.Auth = &Auth{
			Username: proxyURL.User.Username(),
			Password: password,
		}
	}
	return node, nil
}

func localServiceProxyURL(local LocalService) (*url.URL, error) {
	addr := strings.TrimSpace(local.Addr)
	if addr == "" {
		return nil, errors.New("common egress address is required")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			host = "127.0.0.1"
			port = strings.TrimPrefix(addr, ":")
		} else {
			return nil, fmt.Errorf("parse common egress address: %w", err)
		}
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	return &url.URL{
		Scheme: normalizeLocalProtocol(local.Protocol),
		Host:   net.JoinHostPort(host, port),
	}, nil
}

func splitNodeScheme(scheme string) (string, string) {
	scheme = strings.ToLower(strings.TrimSpace(scheme))
	if scheme == "" {
		return "http", "tcp"
	}
	if scheme == "https" {
		return "http", "tls"
	}
	if strings.Contains(scheme, "+") {
		parts := strings.SplitN(scheme, "+", 2)
		return normalizeConnector(parts[0]), normalizeDialer(parts[1])
	}
	return normalizeConnector(scheme), "tcp"
}

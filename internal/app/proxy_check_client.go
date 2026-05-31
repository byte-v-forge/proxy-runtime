package app

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/byte-v-forge/common-lib/httpclient"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

func (r *Runtime) checkProxyHTTPClient(poolID string, providerID string, listenerID string, timeout time.Duration) (*http.Client, error) {
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
	client, err := httpclient.NewWithSchemes(timeout, proxyURL, httpclient.CommonProxySchemes...)
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

package provider

import (
	"errors"
	"net/url"
	"strings"
)

func ParseProxy(raw string, defaultScheme string) (*url.URL, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, errors.New("proxy value is empty")
	}
	scheme := normalizeScheme(defaultScheme)
	if strings.Contains(value, "://") {
		return parseURL(value, scheme)
	}
	if strings.Contains(value, "@") {
		return parseAtFormat(value, scheme)
	}
	if strings.Count(value, ":") == 3 {
		return parseHostPortUserPass(value, scheme)
	}
	return parseURL(scheme+"://"+value, scheme)
}

func parseURL(raw string, defaultScheme string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New("invalid proxy URL")
	}
	if parsed.Scheme == "" {
		parsed.Scheme = normalizeScheme(defaultScheme)
	}
	if parsed.Host == "" {
		return nil, errors.New("proxy URL host is required")
	}
	return parsed, nil
}

func parseAtFormat(value string, scheme string) (*url.URL, error) {
	parts := strings.Split(value, "@")
	if len(parts) != 2 {
		return nil, errors.New("invalid proxy credential format")
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if strings.Count(left, ":") == 1 && strings.Count(right, ":") == 1 {
		hostPort := left
		userPass := strings.SplitN(right, ":", 2)
		return buildURL(scheme, hostPort, userPass[0], userPass[1])
	}
	return parseURL(scheme+"://"+value, scheme)
}

func parseHostPortUserPass(value string, scheme string) (*url.URL, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return nil, errors.New("invalid host:port:user:password proxy format")
	}
	return buildURL(scheme, parts[0]+":"+parts[1], parts[2], parts[3])
}

func buildURL(scheme string, host string, username string, password string) (*url.URL, error) {
	if strings.TrimSpace(host) == "" {
		return nil, errors.New("proxy host is required")
	}
	return &url.URL{
		Scheme: normalizeScheme(scheme),
		Host:   strings.TrimSpace(host),
		User:   url.UserPassword(strings.TrimSpace(username), strings.TrimSpace(password)),
	}, nil
}

func normalizeScheme(scheme string) string {
	switch strings.ToLower(strings.TrimSpace(scheme)) {
	case "socks5", "socks5h":
		return "socks5"
	case "https":
		return "https"
	case "http", "":
		return "http"
	default:
		return strings.ToLower(strings.TrimSpace(scheme))
	}
}

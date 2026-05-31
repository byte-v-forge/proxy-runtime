package gost

import (
	"strings"
)

func buildService(local LocalService, fallbackName string, chainName string) Service {
	name := strings.TrimSpace(local.Name)
	if name == "" {
		name = fallbackName
	}
	handler := Handler{Type: normalizeLocalProtocol(local.Protocol)}
	if chainName != "" {
		handler.Chain = chainName
	}
	if local.Username != "" || local.Password != "" {
		handler.Auth = &Auth{Username: local.Username, Password: local.Password}
	}
	return Service{
		Name:     name,
		Addr:     local.Addr,
		Handler:  handler,
		Listener: Listener{Type: "tcp"},
	}
}

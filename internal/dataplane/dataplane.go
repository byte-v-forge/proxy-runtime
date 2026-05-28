package dataplane

import (
	"context"
	"net/url"

	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

type Driver interface {
	Name() string
	ReconcileBase(ctx context.Context, cfg Config) error
	UpsertSessionRoute(ctx context.Context, route SessionRoute) error
	DeleteSessionRoute(ctx context.Context, route SessionRoute) error
	Stop()
	Status() Status
}

type Status struct {
	Running    bool
	ConfigPath string
	LastError  string
}

type Config struct {
	Common           *LocalService
	Local            LocalService
	Listeners        []LocalService
	StaticChain      []*url.URL
	Pool             []provider.Node
	DynamicViaCommon bool
}

type LocalService struct {
	Name     string
	Addr     string
	Protocol string
	Username string
	Password string
	Route    string
	Upstream string
}

type SessionRoute struct {
	SessionID   string
	ChainID     string
	Listener    LocalService
	StaticChain []*url.URL
	Pool        []provider.Node
}

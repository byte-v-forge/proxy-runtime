package gost

import (
	"net/url"

	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

type Config struct {
	Services []Service `json:"services"`
	Chains   []Chain   `json:"chains,omitempty"`
}

type Service struct {
	Name     string   `json:"name"`
	Addr     string   `json:"addr"`
	Handler  Handler  `json:"handler"`
	Listener Listener `json:"listener"`
}

type Handler struct {
	Type  string `json:"type"`
	Chain string `json:"chain,omitempty"`
	Auth  *Auth  `json:"auth,omitempty"`
}

type Listener struct {
	Type string `json:"type"`
}

type Chain struct {
	Name string `json:"name"`
	Hops []Hop  `json:"hops"`
}

type Hop struct {
	Name  string `json:"name"`
	Nodes []Node `json:"nodes"`
}

type Node struct {
	Name      string    `json:"name"`
	Addr      string    `json:"addr"`
	Connector Connector `json:"connector"`
	Dialer    Dialer    `json:"dialer"`
}

type Connector struct {
	Type string `json:"type"`
	Auth *Auth  `json:"auth,omitempty"`
}

type Dialer struct {
	Type string `json:"type"`
}

type Auth struct {
	Username string `json:"username"`
	Password string `json:"password"`
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

type EgressConfig struct {
	Common           *LocalService
	Local            LocalService
	Listeners        []LocalService
	StaticChain      []*url.URL
	Pool             []provider.Node
	DynamicViaCommon bool
}

type SessionRoute struct {
	SessionID string
	Listener  LocalService
	Pool      []provider.Node
}

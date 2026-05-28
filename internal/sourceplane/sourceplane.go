package sourceplane

import (
	"context"
	"errors"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
)

type Driver interface {
	Name() string
	Reconcile(ctx context.Context, cfg Config) ([]provider.Node, error)
	Sources(ctx context.Context) ([]*proxyruntimev1.ProxySourceDescriptor, error)
	SourceNodes(ctx context.Context, sourceID string) ([]*proxyruntimev1.ProxySourceNode, error)
	UpsertSubscriptionSource(ctx context.Context, req *proxyruntimev1.UpsertProxySubscriptionSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error)
	UpsertFixedSource(ctx context.Context, req *proxyruntimev1.UpsertProxyFixedSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error)
	DeleteSource(ctx context.Context, sourceID string) error
	Stop()
	Status() Status
}

type Status struct {
	Running    bool
	ConfigPath string
	LastError  string
}

type Config struct {
	Providers           []SubscriptionProvider
	Endpoint            Endpoint
	GroupStrategy       string
	HealthCheckURL      string
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
}

type Endpoint struct {
	Addr     string
	Protocol string
}

type FixedProxy struct {
	ID          string
	DisplayName string
	URI         string
	RegionCodes []string
}

type SubscriptionProvider struct {
	ID             string
	DisplayName    string
	URL            string
	Path           string
	Filter         string
	ExcludeFilter  string
	Interval       time.Duration
	HealthCheckURL string
	HealthInterval time.Duration
	HealthTimeout  time.Duration
	HealthLazy     bool
	ExpectedStatus uint32
	RegionCodes    []string
	Headers        map[string][]string
}

type Empty struct{}

func (Empty) Name() string                                               { return "none" }
func (Empty) Reconcile(context.Context, Config) ([]provider.Node, error) { return nil, nil }
func (Empty) Sources(context.Context) ([]*proxyruntimev1.ProxySourceDescriptor, error) {
	return nil, nil
}
func (Empty) SourceNodes(context.Context, string) ([]*proxyruntimev1.ProxySourceNode, error) {
	return nil, nil
}
func (Empty) UpsertSubscriptionSource(context.Context, *proxyruntimev1.UpsertProxySubscriptionSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error) {
	return nil, errors.New("source runtime is disabled")
}
func (Empty) UpsertFixedSource(context.Context, *proxyruntimev1.UpsertProxyFixedSourceRequest) (*proxyruntimev1.ProxySourceDescriptor, error) {
	return nil, errors.New("source runtime is disabled")
}
func (Empty) DeleteSource(context.Context, string) error {
	return errors.New("source runtime is disabled")
}
func (Empty) Stop()          {}
func (Empty) Status() Status { return Status{LastError: "disabled"} }

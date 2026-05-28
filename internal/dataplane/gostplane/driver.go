package gostplane

import (
	"context"
	"log/slog"

	"github.com/byte-v-forge/proxy-runtime/internal/dataplane"
	"github.com/byte-v-forge/proxy-runtime/internal/gost"
)

type Driver struct{ manager *gost.Manager }

func New(cfg gost.ManagerConfig, logger *slog.Logger) *Driver {
	return &Driver{manager: gost.NewManager(cfg, logger)}
}
func (d *Driver) Name() string { return "gost" }

func (d *Driver) ReconcileBase(ctx context.Context, cfg dataplane.Config) error {
	gostConfig, err := gost.BuildEgressConfig(gost.EgressConfig{Common: toGostLocalPtr(cfg.Common), Local: toGostLocal(cfg.Local), Listeners: toGostLocals(cfg.Listeners), StaticChain: cfg.StaticChain, Pool: cfg.Pool, DynamicViaCommon: cfg.DynamicViaCommon})
	if err != nil {
		return err
	}
	return d.manager.Reload(ctx, gostConfig)
}

func (d *Driver) UpsertSessionRoute(ctx context.Context, route dataplane.SessionRoute) error {
	service, chain, err := gost.BuildSessionRoute(route.ChainID, toGostLocal(route.Listener), route.StaticChain, route.Pool)
	if err != nil {
		return err
	}
	return d.manager.UpsertRoute(ctx, service, chain)
}

func (d *Driver) DeleteSessionRoute(ctx context.Context, route dataplane.SessionRoute) error {
	service, chain, err := gost.BuildSessionRoute(route.ChainID, toGostLocal(route.Listener), route.StaticChain, route.Pool)
	if err != nil {
		return err
	}
	return d.manager.DeleteRoute(ctx, service.Name, chain.Name)
}

func (d *Driver) Stop() { d.manager.Stop() }
func (d *Driver) Status() dataplane.Status {
	status := d.manager.Status()
	return dataplane.Status{Running: status.Running, ConfigPath: status.ConfigPath, LastError: status.LastError}
}

func toGostLocalPtr(in *dataplane.LocalService) *gost.LocalService {
	if in == nil {
		return nil
	}
	out := toGostLocal(*in)
	return &out
}
func toGostLocals(in []dataplane.LocalService) []gost.LocalService {
	out := make([]gost.LocalService, 0, len(in))
	for _, item := range in {
		out = append(out, toGostLocal(item))
	}
	return out
}
func toGostLocal(in dataplane.LocalService) gost.LocalService {
	return gost.LocalService{Name: in.Name, Addr: in.Addr, Protocol: in.Protocol, Username: in.Username, Password: in.Password, Route: in.Route, Upstream: in.Upstream}
}

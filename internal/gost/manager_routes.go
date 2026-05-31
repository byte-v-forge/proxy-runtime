package gost

import (
	"context"
	"errors"
	"time"
)

func (m *Manager) UpsertRoute(ctx context.Context, service Service, chain Chain) error {
	if m.cfg.APIAddr == "" {
		return errors.New("gost api is required for dynamic route update")
	}
	if len(chain.Hops) > 0 {
		if err := m.putOrCreate(ctx, "chains", chain.Name, chain); err != nil {
			return err
		}
	}
	if err := m.putOrCreate(ctx, "services", service.Name, service); err != nil {
		return err
	}
	return waitForServices(ctx, []Service{service}, 3*time.Second)
}

func (m *Manager) DeleteRoute(ctx context.Context, serviceName string, chainName string) error {
	if m.cfg.APIAddr == "" {
		return errors.New("gost api is required for dynamic route delete")
	}
	if serviceName != "" {
		if err := m.deleteConfigObject(ctx, "services", serviceName); err != nil {
			return err
		}
	}
	if chainName != "" {
		if err := m.deleteConfigObject(ctx, "chains", chainName); err != nil {
			return err
		}
	}
	return nil
}

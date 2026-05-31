package app

import (
	"context"
	"errors"
	"net/http"
)

func (r *Runtime) lookupIPGeo(ctx context.Context, ip string) (proxyExitGeo, error) {
	if geo, ok := r.cachedIPGeo(ip); ok {
		return geo, nil
	}
	settings, err := r.settings.load()
	if err != nil {
		return proxyExitGeo{}, err
	}
	timeout := proxyExitIPTimeout(settings)
	lookupCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	client := &http.Client{Timeout: timeout}
	geo, err := firstSuccessfulIPGeo(lookupCtx, client, ipGeoLookupEndpoints(ip))
	if err != nil {
		return proxyExitGeo{}, err
	}
	geo.IP = ip
	r.saveIPGeoCache(ip, geo)
	return geo, nil
}

func firstSuccessfulIPGeo(ctx context.Context, client *http.Client, endpoints []string) (proxyExitGeo, error) {
	if len(endpoints) == 0 {
		return proxyExitGeo{}, errors.New("ip geo endpoint unavailable")
	}
	type geoResult struct {
		geo proxyExitGeo
		err error
	}
	results := make(chan geoResult, len(endpoints))
	for _, endpoint := range endpoints {
		endpoint := endpoint
		go func() {
			geo, err := requestIPInfo(ctx, client, endpoint, false)
			results <- geoResult{geo: geo, err: err}
		}()
	}
	for range endpoints {
		select {
		case <-ctx.Done():
			return proxyExitGeo{}, errors.New("lookup ip geo timed out")
		case result := <-results:
			if result.err == nil {
				return result.geo, nil
			}
		}
	}
	return proxyExitGeo{}, errors.New("lookup ip geo failed")
}

package app

import (
	"strings"
	"time"
)

func (r *Runtime) cachedIPGeo(ip string) (proxyExitGeo, bool) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return proxyExitGeo{}, false
	}
	r.geoMu.Lock()
	defer r.geoMu.Unlock()
	if r.geoCache == nil {
		return proxyExitGeo{}, false
	}
	item, ok := r.geoCache[ip]
	if !ok || time.Now().After(item.expiresAt) {
		delete(r.geoCache, ip)
		return proxyExitGeo{}, false
	}
	return item.geo, true
}

func (r *Runtime) saveIPGeoCache(ip string, geo proxyExitGeo) {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return
	}
	r.geoMu.Lock()
	defer r.geoMu.Unlock()
	if r.geoCache == nil {
		r.geoCache = map[string]cachedIPGeo{}
	}
	r.geoCache[ip] = cachedIPGeo{geo: geo, expiresAt: time.Now().Add(ipGeoCacheTTL)}
}

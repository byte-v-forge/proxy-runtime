package app

import (
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/proxy-runtime/internal/provider/accountproxy"
)

func (r *Runtime) handleHealth(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Runtime) handleReady(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	status := r.routePlane.Status()
	if !status.Running {
		msg := firstNonEmpty(status.LastError, "route runtime is not running")
		http.Error(w, msg, http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (r *Runtime) handleProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	settings, err := r.settings.load()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyProvidersResponse{Providers: accountproxy.Descriptors(dynamicIPGatewayMap(settings))})
}

func (r *Runtime) handleGateway(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	gateway, err := r.gateway(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetEgressGatewayResponse{Gateway: gateway})
}

func (r *Runtime) handlePool(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	pool, err := r.snapshot(req.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyPoolResponse{Pool: pool})
}

func (r *Runtime) handleRefresh(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if err := r.refresh(req.Context()); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	pool, _ := r.snapshot(req.Context())
	r.writeProto(w, &proxyruntimev1.RefreshProxyPoolResponse{Pool: pool})
}

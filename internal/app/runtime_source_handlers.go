package app

import (
	"context"
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func (r *Runtime) handleSources(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		sources, err := r.listSources(req.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.writeProto(w, &proxyruntimev1.ListProxySourcesResponse{Sources: sources})
	case http.MethodPost, http.MethodPut:
		var body proxyruntimev1.UpsertProxySubscriptionSourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		source, err := r.sourcePlane.UpsertSubscriptionSource(req.Context(), &body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.UpsertProxySubscriptionSourceResponse{Source: source})
	case http.MethodDelete:
		var body proxyruntimev1.DeleteProxySourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		if err := r.sourcePlane.DeleteSource(req.Context(), body.GetSourceId()); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.DeleteProxySourceResponse{})
	default:
		methodNotAllowed(w, http.MethodGet+", "+http.MethodPost+", "+http.MethodPut+", "+http.MethodDelete)
	}
}

func (r *Runtime) handleFixedSources(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
		var body proxyruntimev1.UpsertProxyFixedSourceRequest
		if !r.readProto(w, req, &body) {
			return
		}
		source, err := r.sourcePlane.UpsertFixedSource(req.Context(), &body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		_ = r.refresh(req.Context())
		r.writeProto(w, &proxyruntimev1.UpsertProxyFixedSourceResponse{Source: source})
	default:
		methodNotAllowed(w, http.MethodPost+", "+http.MethodPut)
	}
}

func (r *Runtime) handleSourceNodes(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	nodes, err := r.sourcePlane.SourceNodes(req.Context(), req.URL.Query().Get("source_id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxySourceNodesResponse{Nodes: nodes})
}

func (r *Runtime) handleResolveChain(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.ResolveProxyChainRequest
	if !r.readProto(w, req, &body) {
		return
	}
	response, err := r.resolveProxyChain(req.Context(), &body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, response)
}

func (r *Runtime) listSources(ctx context.Context) ([]*proxyruntimev1.ProxySourceDescriptor, error) {
	settings, err := r.settings.load()
	if err != nil {
		return nil, err
	}
	sources, err := r.store.ListSources(ctx, len(r.cfg.StaticChain), dynamicIPGatewayMap(settings))
	if err != nil {
		return nil, err
	}
	sourcePlaneSources, err := r.sourcePlane.Sources(ctx)
	if err != nil {
		return nil, err
	}
	return append(sources, sourcePlaneSources...), nil
}

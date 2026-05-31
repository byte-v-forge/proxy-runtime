package app

import (
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"github.com/byte-v-forge/proxy-runtime/internal/ipfraud"
)

func (r *Runtime) handleGetProxyExitIP(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.GetProxyExitIPRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	exitIP, err := r.getProxyExitIP(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyExitIPResponse{ProxyExitIp: exitIP})
}

func (r *Runtime) handleGetProxyExitGeo(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.GetProxyExitGeoRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	geo, err := r.getProxyExitGeo(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.GetProxyExitGeoResponse{ProxyExitGeo: geo})
}

func (r *Runtime) handleCheckIPFraud(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyIPFraudRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkProxyIPFraud(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyIPFraudResponse{Check: check})
}

func (r *Runtime) handleCheckEdgeAccessRisk(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyEdgeAccessRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkProxyEdgeAccess(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyEdgeAccessResponse{Check: check})
}

func (r *Runtime) handleCheckTargetConnectivity(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var checkReq proxyruntimev1.CheckProxyTargetConnectivityRequest
	if req.Body != nil && req.ContentLength != 0 {
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &checkReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	check, err := r.checkTargetConnectivity(req.Context(), &checkReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.CheckProxyTargetConnectivityResponse{Check: check})
}

func (r *Runtime) handleIPFraudProviders(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyIPFraudProvidersResponse{Providers: ipfraud.ProviderDescriptors()})
}

func (r *Runtime) handleRuntimeSettings(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodGet:
		settings, err := r.settings.view()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		r.writeProto(w, &proxyruntimev1.GetProxyRuntimeSettingsResponse{Settings: settings})
	case http.MethodPost, http.MethodPut:
		var updateReq proxyruntimev1.UpdateProxyRuntimeSettingsRequest
		if req.Body == nil {
			http.Error(w, "request body is required", http.StatusBadRequest)
			return
		}
		body, err := readRequestBody(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := protojsonx.Unmarshal(body, &updateReq); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		settings, err := r.settings.update(&updateReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		r.resetIPFraudChecker()
		r.writeProto(w, &proxyruntimev1.UpdateProxyRuntimeSettingsResponse{Settings: settings})
	default:
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodPost+", "+http.MethodPut)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

package app

import (
	"net/http"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
)

func (r *Runtime) handleLeases(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	leases, err := r.leases.ListLeases(req.Context(), false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	r.writeProto(w, &proxyruntimev1.ListProxyDynamicLeasesResponse{Leases: leases})
}

func (r *Runtime) handleAcquireLease(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.AcquireProxyLeaseRequest
	if !r.readProto(w, req, &body) {
		return
	}
	lease, err := r.acquireLease(req.Context(), req, &body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	pool, _ := r.snapshot(req.Context())
	r.writeProto(w, &proxyruntimev1.AcquireProxyLeaseResponse{Lease: lease, Pool: pool, Egress: lease.GetEgress(), ChainPlan: lease.GetChainPlan()})
}

func (r *Runtime) handleReleaseLease(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	var body proxyruntimev1.ReleaseProxyLeaseRequest
	if !r.readProto(w, req, &body) {
		return
	}
	lease, err := r.releaseLease(req.Context(), body.GetAccountId())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	r.writeProto(w, &proxyruntimev1.ReleaseProxyLeaseResponse{Lease: lease})
}

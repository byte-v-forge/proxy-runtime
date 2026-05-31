package app

import (
	"net/http"
	"strings"

	"github.com/byte-v-forge/common-lib/httpx"
	"github.com/byte-v-forge/common-lib/protojsonx"
	"google.golang.org/protobuf/proto"
)

func (r *Runtime) readProto(w http.ResponseWriter, req *http.Request, message proto.Message) bool {
	if req.Body == nil {
		http.Error(w, "request body is required", http.StatusBadRequest)
		return false
	}
	body, err := readRequestBody(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true
	}
	if err := protojsonx.Unmarshal(body, message); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func (r *Runtime) writeProto(w http.ResponseWriter, message proto.Message) {
	data, err := protojsonx.Marshal(message)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

func readRequestBody(req *http.Request) ([]byte, error) {
	defer req.Body.Close()
	return httpx.ReadLimited(req.Body, 1<<20)
}

func methodNotAllowed(w http.ResponseWriter, allow string) {
	w.Header().Set("Allow", allow)
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

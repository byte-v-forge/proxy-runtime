package gost

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func (m *Manager) putOrCreate(ctx context.Context, kind string, name string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	status, text, err := m.request(ctx, http.MethodPut, fmt.Sprintf("/config/%s/%s", kind, name), body)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if !isConfigObjectNotFound(status, text) {
		return fmt.Errorf("gost update %s/%s returned HTTP %d: %s", kind, name, status, text)
	}
	status, text, err = m.request(ctx, http.MethodPost, "/config/"+kind, body)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("gost create %s/%s returned HTTP %d: %s", kind, name, status, text)
}

func (m *Manager) deleteConfigObject(ctx context.Context, kind string, name string) error {
	status, text, err := m.request(ctx, http.MethodDelete, fmt.Sprintf("/config/%s/%s", kind, name), nil)
	if err != nil {
		return err
	}
	if isConfigObjectNotFound(status, text) || status == http.StatusNoContent || status == http.StatusOK {
		return nil
	}
	if status >= 200 && status < 300 {
		return nil
	}
	return fmt.Errorf("gost delete %s/%s returned HTTP %d: %s", kind, name, status, text)
}

func isConfigObjectNotFound(status int, text string) bool {
	if status == http.StatusNotFound || status == http.StatusMethodNotAllowed {
		return true
	}
	if status != http.StatusBadRequest {
		return false
	}
	text = strings.ToLower(text)
	return strings.Contains(text, "40004") || strings.Contains(text, "not found")
}

func (m *Manager) request(ctx context.Context, method string, path string, body []byte) (int, string, error) {
	url := "http://" + m.cfg.APIAddr + path
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return 0, "", err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := m.apiClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, string(respBody), nil
}

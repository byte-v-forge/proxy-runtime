package ten24

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	proxyruntimev1 "github.com/byte-v-forge/common-lib/gen/go/byte/v/forge/contracts/proxyruntime/v1"
	"github.com/byte-v-forge/common-lib/randx"
	"github.com/byte-v-forge/proxy-runtime/internal/provider"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (p *Provider) Fetch(ctx context.Context, session *proxyruntimev1.ProxySession) ([]provider.Node, error) {
	if session == nil {
		if p.cfg.APIURL != "" {
			return p.fetchAPI(ctx)
		}
		if !p.supportsCredentialSession() {
			return nil, nil
		}
	}
	node, err := p.credentialNode(session)
	if err != nil {
		return nil, err
	}
	return []provider.Node{node}, nil
}

func (p *Provider) CreateSession(_ context.Context, req *proxyruntimev1.AcquireProxyLeaseRequest) (*proxyruntimev1.ProxySession, error) {
	if !p.supportsCredentialSession() {
		return nil, provider.ErrUnsupportedCapability
	}
	policy := p.sessionPolicy(req.GetPolicy())
	if policy.Mode == proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		policy.Mode = proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY
	}
	if policy.Mode != proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY {
		return nil, provider.ErrUnsupportedCapability
	}
	policy.UpstreamKind = proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP
	policy.RotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	if policy.Asn != "" && (policy.State != "" || policy.City != "") {
		return nil, errors.New("1024proxy ASN cannot be combined with state or city")
	}
	sessionID, err := randx.Hex(8)
	if err != nil {
		return nil, fmt.Errorf("generate 1024proxy session id: %w", err)
	}
	now := time.Now().UTC()
	stickyTTL := policyStickyTTL(policy)
	return &proxyruntimev1.ProxySession{
		SessionId:  sessionID,
		ProviderId: p.Name(),
		Policy:     policy,
		CreatedAt:  timestamppb.New(now),
		ExpiresAt:  timestamppb.New(now.Add(stickyTTL)),
		AccountId:  strings.TrimSpace(req.GetAccountId()),
		Purpose:    strings.TrimSpace(req.GetPurpose()),
		Labels: map[string]string{
			"provider":     p.Name(),
			"session_mode": "username_parameter",
		},
	}, nil
}

func (p *Provider) credentialNode(session *proxyruntimev1.ProxySession) (provider.Node, error) {
	username := p.buildUsername(session)
	proxyURL := url.URL{
		Scheme: defaultProtocol(p.cfg.Protocol),
		Host:   p.cfg.ProxyAddr,
		User:   url.UserPassword(username, p.cfg.Password),
	}
	sessionID := ""
	if session != nil {
		sessionID = session.SessionId
	} else {
		sessionID = p.cfg.SessionID
	}
	rotationMode := proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_PER_REQUEST
	if sessionID != "" {
		rotationMode = proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION
	}
	return provider.Node{
		ID:           "1024proxy-credential",
		URL:          &proxyURL,
		ProviderID:   p.Name(),
		SessionID:    sessionID,
		UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP,
		RotationMode: rotationMode,
		Labels: map[string]string{
			"mode":         "credential",
			"network_kind": "residential",
			"protocol":     defaultProtocol(p.cfg.Protocol),
		},
	}, nil
}

func (p *Provider) buildUsername(session *proxyruntimev1.ProxySession) string {
	policy := p.sessionPolicy(nil)
	sessionID := p.cfg.SessionID
	if session != nil {
		policy = p.sessionPolicy(session.Policy)
		sessionID = session.SessionId
	}
	parts := []string{p.cfg.Username}
	appendPair := func(key string, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			parts = append(parts, key, value)
		}
	}
	appendPair("region", policy.Region)
	appendPair("st", policy.State)
	appendPair("city", policy.City)
	appendPair("asn", policy.Asn)
	if sessionID != "" {
		appendPair("sid", sessionID)
		appendPair("t", strconv.Itoa(policyStickyMinutes(policy)))
	}
	return strings.Join(parts, "-")
}

func (p *Provider) sessionPolicy(input *proxyruntimev1.ProxySessionPolicy) *proxyruntimev1.ProxySessionPolicy {
	policy := &proxyruntimev1.ProxySessionPolicy{
		Mode:         proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_STICKY,
		StickyTtl:    stickyDuration(defaultStickyTTL(p.cfg.StickyMinutes)),
		UpstreamKind: proxyruntimev1.ProxyUpstreamKind_PROXY_UPSTREAM_KIND_DYNAMIC_IP,
		RotationMode: proxyruntimev1.ProxyRotationMode_PROXY_ROTATION_MODE_STICKY_SESSION,
	}
	if input == nil {
		return policy
	}
	if input.Mode != proxyruntimev1.ProxySessionMode_PROXY_SESSION_MODE_UNSPECIFIED {
		policy.Mode = input.Mode
	}
	if input.Region != "" {
		policy.Region = input.Region
	}
	if input.State != "" {
		policy.State = input.State
	}
	if input.City != "" {
		policy.City = input.City
	}
	if input.Asn != "" {
		policy.Asn = input.Asn
	}
	if stickyMinutes := durationMinutes(input.GetStickyTtl()); stickyMinutes > 0 {
		policy.StickyTtl = stickyDuration(stickyMinutes)
	}
	policy.StickyTtl = stickyDuration(policyStickyMinutes(policy))
	if len(input.Labels) > 0 {
		policy.Labels = make(map[string]string, len(input.Labels))
		for key, value := range input.Labels {
			policy.Labels[key] = value
		}
	}
	return policy
}

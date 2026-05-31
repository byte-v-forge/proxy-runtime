package app

import (
	"context"
	"io"
	"net"
	"strings"
	"time"

	"github.com/byte-v-forge/common-lib/proxyurl"
	"github.com/byte-v-forge/proxy-runtime/internal/config"
)

func (r *Runtime) reloadForwarders(_ context.Context) error {
	listeners := r.upstreamListenerConfigs()
	signature := upstreamForwarderSignature(listeners)
	r.forwardMu.Lock()
	if r.forwardSignature == signature {
		r.forwardMu.Unlock()
		return nil
	}
	r.forwardMu.Unlock()
	r.stopForwarders()
	if len(listeners) == 0 {
		r.forwardMu.Lock()
		r.forwardSignature = signature
		r.forwardMu.Unlock()
		return nil
	}
	forwardCtx, cancel := context.WithCancel(context.Background())
	started := make([]net.Listener, 0, len(listeners))
	for _, listener := range listeners {
		upstream, err := proxyurl.Parse(listener.Upstream, "socks5")
		if err != nil {
			closeListeners(started)
			cancel()
			return err
		}
		ln, err := net.Listen("tcp", listener.Addr)
		if err != nil {
			closeListeners(started)
			cancel()
			return err
		}
		started = append(started, ln)
		go r.serveForwarder(forwardCtx, listener.ID, ln, upstream.Host)
	}
	r.forwardMu.Lock()
	r.forwardCancel = cancel
	r.forwardListeners = started
	r.forwardSignature = signature
	r.forwardMu.Unlock()
	return nil
}

func (r *Runtime) stopForwarders() {
	r.forwardMu.Lock()
	cancel := r.forwardCancel
	listeners := r.forwardListeners
	r.forwardCancel = nil
	r.forwardListeners = nil
	r.forwardSignature = ""
	r.forwardMu.Unlock()
	if cancel != nil {
		cancel()
	}
	closeListeners(listeners)
}

func closeListeners(listeners []net.Listener) {
	for _, ln := range listeners {
		_ = ln.Close()
	}
}

func upstreamForwarderSignature(listeners []config.EgressListener) string {
	var b strings.Builder
	for _, listener := range listeners {
		b.WriteString(listener.ID)
		b.WriteByte('\x00')
		b.WriteString(listener.Addr)
		b.WriteByte('\x00')
		b.WriteString(listener.Upstream)
		b.WriteByte('\x00')
	}
	return b.String()
}

func (r *Runtime) serveForwarder(ctx context.Context, listenerID string, ln net.Listener, upstreamAddr string) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		go r.forwardConn(ctx, listenerID, conn, upstreamAddr)
	}
}

func (r *Runtime) forwardConn(ctx context.Context, _ string, client net.Conn, upstreamAddr string) {
	defer client.Close()
	upstream, err := (&net.Dialer{Timeout: 10 * time.Second}).DialContext(ctx, "tcp", upstreamAddr)
	if err != nil {
		return
	}
	defer upstream.Close()
	done := make(chan struct{}, 2)
	copyConn := func(dst net.Conn, src net.Conn) { _, _ = io.Copy(dst, src); _ = dst.Close(); done <- struct{}{} }
	go copyConn(upstream, client)
	go copyConn(client, upstream)
	select {
	case <-ctx.Done():
	case <-done:
	}
}

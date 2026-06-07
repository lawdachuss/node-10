package internal

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/sardanioss/httpcloak"
	"github.com/teacat/chaturbate-dvr/server"
)

// httpcloakTransport wraps httpcloak.Client as an http.RoundTripper.
// It emulates a Chrome 146 TLS/HTTP2 fingerprint to bypass Cloudflare WAF
// TCP RST that Go's default crypto/tls triggers.
// ECH (Encrypted Client Hello) hides the SNI from network observers for
// better Cloudflare bot scores.
type httpcloakTransport struct {
	client *httpcloak.Client
	mu     sync.Mutex
}

var sharedCloakTransport = sync.OnceValue(func() http.RoundTripper {
	opts := []httpcloak.Option{
		httpcloak.WithTimeout(30 * time.Second),
	}
	if proxyURL := configuredProxyURL(); proxyURL != "" {
		opts = append(opts, httpcloak.WithProxy(proxyURL))
	}
	c := httpcloak.New("chrome-146-windows", opts...)
	return &httpcloakTransport{client: c}
})

func configuredProxyURL() string {
	if server.Config == nil {
		return ""
	}
	proxyURL := strings.TrimSpace(server.Config.ProxyURL)
	if proxyURL == "" {
		return ""
	}
	username := strings.TrimSpace(server.Config.ProxyUsername)
	password := strings.TrimSpace(server.Config.ProxyPassword)
	if username == "" && password == "" {
		return proxyURL
	}

	u, err := url.Parse(proxyURL)
	if err != nil || u.Scheme == "" || u.Host == "" || u.User != nil {
		return proxyURL
	}
	if password != "" {
		u.User = url.UserPassword(username, password)
	} else {
		u.User = url.User(username)
	}
	return u.String()
}

// WarmupChaturbate makes an initial request to chaturbate.com to establish
// TLS session tickets with Cloudflare before any API calls are made.
// This gives subsequent requests TLS session resumption, making them look
// more like a returning browser visitor.
func WarmupChaturbate(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://chaturbate.com/", nil)
	if err != nil {
		return
	}
	SetRequestHeaders(req)
	resp, err := sharedCloakTransport().RoundTrip(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

// WarmupStripchat makes an initial request to stripchat.com to establish TLS
// session tickets before any API calls are made. This is the same idea as
// WarmupChaturbate but for Stripchat's domain.
func WarmupStripchat(ctx context.Context) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", "https://stripchat.com/", nil)
	if err != nil {
		return
	}
	SetRequestHeaders(req)
	resp, err := sharedCloakTransport().RoundTrip(req)
	if err != nil {
		return
	}
	resp.Body.Close()
}

func (t *httpcloakTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme == "http" {
		return http.DefaultTransport.RoundTrip(req)
	}

	ctx := req.Context()
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	var bodyBytes []byte
	if req.Body != nil {
		var err error
		bodyBytes, err = io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
	}

	cloakReq := &httpcloak.Request{
		Method:  req.Method,
		URL:     req.URL.String(),
		Headers: req.Header,
	}

	if len(bodyBytes) > 0 {
		cloakReq.Body = bytes.NewReader(bodyBytes)
	}

	cloakResp, err := t.client.Do(ctx, cloakReq)
	if err != nil {
		return nil, err
	}

	body, err := cloakResp.Bytes()
	if err != nil {
		cloakResp.Close()
		return nil, err
	}

	resp := &http.Response{
		StatusCode: cloakResp.StatusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}

	if cloakResp.Headers != nil {
		for k, vs := range cloakResp.Headers {
			for _, v := range vs {
				resp.Header.Add(k, v)
			}
		}
	}

	return resp, nil
}

package http_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "go.adoublef/blob/internal/net/http"
	"go.adoublef/blob/internal/net/http/httputil"
	"go.adoublef/blob/internal/net/nettest"
)

type TestClient struct {
	*http.Client
	*nettest.Proxy
	testing.TB
}

func (tc *TestClient) Do(ctx context.Context, pattern string, body io.Reader, opts ...func(*http.Request)) (*http.Response, error) {
	method, _, path, err := httputil.ParsePattern(pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pattern: %v", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to return request: %v", err)
	}
	tc.Logf(`req, err := http.NewRequestWithContext(ctx, %q, %q, body)`, method, path)
	for _, o := range opts {
		o(req)
	}
	res, err := tc.Client.Do(req)
	tc.Logf(`resp, %v := tc.Client.Do(req)`, err)
	return res, err
}

func newTestClient(tb testing.TB, h http.Handler) *TestClient {
	tb.Helper()

	ts := httptest.NewUnstartedServer(h)
	// note: the client panics if readTimeout is less than the test timeout
	// is this a non-issue?
	ts.Config.MaxHeaderBytes = DefaultMaxHeaderBytes
	ts.Config.ReadTimeout = DefaultReadTimeout
	ts.Config.WriteTimeout = DefaultWriteTimeout
	ts.Config.IdleTimeout = DefaultIdleTimeout
	ts.StartTLS()
	proxy := nettest.NewProxy(tb.Name(), strings.TrimPrefix(ts.URL, "https://"))
	tc := nettest.WithTransport(ts.Client(), "https://"+proxy.Listen())
	return &TestClient{tc, proxy, tb}
}

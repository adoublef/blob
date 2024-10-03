package http_test

import (
	"context"
	"embed"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "go.adoublef/blob/internal/net/http"
	"go.adoublef/blob/internal/net/http/httputil"
	"go.adoublef/blob/internal/net/nettest"
)

//go:embed all:testdata/*
var embedFS embed.FS

type TestClient struct {
	*http.Client
	*nettest.Proxy
	testing.TB
}

func (tc *TestClient) PostFile(ctx context.Context, pattern string, filename string) (*http.Response, error) {
	f, err := embedFS.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to return file stat: %v", err)
	}

	pr, pw := io.Pipe()
	defer pr.Close()

	mw := multipart.NewWriter(pw)
	go func() {
		defer pw.Close()
		defer mw.Close()

		part, err := mw.CreateFormFile("file", fi.Name())
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		// upload a single file
		// can this be extended to upload a directory?
		n, err := io.CopyN(part, f, fi.Size())
		tc.Logf(`%d, %v := io.CopyN(part, f, fi.Size())`, n, err)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
	}()

	o := func(r *http.Request) {
		// set contentType
		r.Header.Set("Content-Type", mw.FormDataContentType())
		// set accept
		r.Header.Set("Accept", "*/*")
		// set encoding?
	}
	return tc.Do(ctx, pattern, pr, o)
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
	tc.Logf(`res, %v := tc.Client.Do(req)`, err)
	return res, err
}

func newTestClient(tb testing.TB, h http.Handler) *TestClient {
	tb.Helper()

	ts := httptest.NewUnstartedServer(h)
	ts.Config.MaxHeaderBytes = DefaultMaxHeaderBytes
	// note: the client panics if readTimeout is less than the test timeout
	// is this a non-issue?
	ts.Config.ReadTimeout = DefaultReadTimeout
	ts.Config.WriteTimeout = DefaultWriteTimeout
	ts.Config.IdleTimeout = DefaultIdleTimeout
	ts.StartTLS()
	proxy := nettest.NewProxy(tb.Name(), strings.TrimPrefix(ts.URL, "https://"))
	tc := nettest.WithTransport(ts.Client(), "https://"+proxy.Listen())
	return &TestClient{tc, proxy, tb}
}

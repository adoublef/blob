package http_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/Shopify/toxiproxy/v2/toxics"
	. "go.adoublef/blob/internal/net/http"
	"go.adoublef/blob/internal/testing/is"
)

var acceptAll = func(r *http.Request) { r.Header.Set("Accept", "*/*") }

func Test_handleUploadBlob(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.PostFile(ctx, "POST /upload-blob", "testdata/hello.txt")
		is.OK(t, err) // return echo response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func Test_handleReady(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		c, ctx := newClient(t), context.Background()

		res, err := c.Do(ctx, "GET /ready", nil, acceptAll)
		is.OK(t, err) // return echo response
		is.Equal(t, res.StatusCode, http.StatusOK)
	})
}

func newClient(tb testing.TB) *TestClient {
	tb.Helper()

	var (
		up = newTestUploader(tb)
	)

	tc := newTestClient(tb, Handler(up))
	// https://speed.cloudflare.com/
	bu, err := tc.AddToxic("bandwidth", true, &toxics.BandwidthToxic{Rate: 72.8 * 1000})
	is.OK(tb, err) // return bandwidth upstream toxic
	lu, err := tc.AddToxic("latency", true, &toxics.LatencyToxic{Latency: 150, Jitter: 42})
	is.OK(tb, err) // return bandwidth upstream toxic
	bd, err := tc.AddToxic("bandwidth", false, &toxics.BandwidthToxic{Rate: 18.4 * 1000})
	is.OK(tb, err) // return bandwidth upstream toxic
	ld, err := tc.AddToxic("latency", false, &toxics.LatencyToxic{Latency: 30, Jitter: 8})
	is.OK(tb, err) // return bandwidth upstream toxic

	tb.Cleanup(func() {
		for _, name := range []string{bu, lu, bd, ld} {
			err := tc.RemoveToxic(name)
			tb.Logf(`%v := tc.RemoveToxic(%q)`, err, name)
		}
	})

	return tc
}
